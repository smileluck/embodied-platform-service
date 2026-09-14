// Package session 会话仓储 —— Redis 实现。
//
// Key 设计（会话状态全量存 Redis，TTL 自动过期，无需清理协程）：
//
//	sess:{sid}          HASH    会话主体，TTL = 会话生命周期（refresh 时续期）
//	udx:{uid}:{device}  STRING  同端互斥键，值为该端当前 sid（登录时顶掉旧会话）
//	uidx:{uid}          SET     用户全部会话索引（踢人下线 / 改密联动用）
//	sess:idx            SET     全局会话索引（在线列表用；过期条目读取时惰性清理）
package session

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	bizsession "github.com/smilex/smilex-admin-gin/internal/biz/session"
)

const keySessionIndex = "sess:idx" // 全局会话索引

// luaReleaseDevice 仅当互斥键仍指向本会话时删除（避免误删新登录顶替后的键）
const luaReleaseDevice = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

func sessionKey(sid string) string { return "sess:" + sid }

// touchKey 会话最近活跃节流水位键
func touchKey(sid string) string { return "sess:touch:" + sid }

func deviceKey(uid uint, device string) string {
	return "udx:" + strconv.FormatUint(uint64(uid), 10) + ":" + device
}

func userIndexKey(uid uint) string {
	return "uidx:" + strconv.FormatUint(uint64(uid), 10)
}

type repo struct {
	client *redis.Client
}

// NewRepo 会话仓储（Redis 实现）
func NewRepo(client *redis.Client) bizsession.Repo {
	return &repo{client: client}
}

func (r *repo) Save(ctx context.Context, s *bizsession.Session, ttl time.Duration) error {
	devKey := deviceKey(s.UserID, s.Device)
	// 同端互斥：该端已有旧会话则随本次登录一并吊销
	oldSid, _ := r.client.Get(ctx, devKey).Result()

	pipe := r.client.TxPipeline()
	if oldSid != "" && oldSid != s.ID {
		pipe.Del(ctx, sessionKey(oldSid))
		pipe.SRem(ctx, keySessionIndex, oldSid)
		pipe.SRem(ctx, userIndexKey(s.UserID), oldSid)
	}
	pipe.HSet(ctx, sessionKey(s.ID), map[string]interface{}{
		"uid": s.UserID, "username": s.Username, "nickname": s.Nickname,
		"device": s.Device, "ip": s.IP, "ua": s.UserAgent,
		"login_at": s.LoginAt.Unix(), "last_active": s.LastActiveAt.Unix(),
	})
	pipe.Expire(ctx, sessionKey(s.ID), ttl)
	pipe.Set(ctx, devKey, s.ID, ttl)
	pipe.SAdd(ctx, keySessionIndex, s.ID)
	pipe.SAdd(ctx, userIndexKey(s.UserID), s.ID)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *repo) Find(ctx context.Context, sid string) (*bizsession.Session, error) {
	fields, err := r.client.HGetAll(ctx, sessionKey(sid)).Result()
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, bizsession.ErrSessionNotFound
	}
	return sessionFromFields(sid, fields), nil
}

func (r *repo) ListAll(ctx context.Context) ([]*bizsession.Session, error) {
	sids, err := r.client.SMembers(ctx, keySessionIndex).Result()
	if err != nil {
		return nil, err
	}
	if len(sids) == 0 {
		return []*bizsession.Session{}, nil
	}
	// pipeline 批量读会话主体
	cmds := make([]*redis.MapStringStringCmd, 0, len(sids))
	pipe := r.client.Pipeline()
	for _, sid := range sids {
		cmds = append(cmds, pipe.HGetAll(ctx, sessionKey(sid)))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	sessions := make([]*bizsession.Session, 0, len(sids))
	for i, cmd := range cmds {
		fields, err := cmd.Result()
		if err != nil || len(fields) == 0 {
			// 会话已过期（TTL 自动清除），索引条目惰性清理
			_ = r.client.SRem(ctx, keySessionIndex, sids[i]).Err()
			continue
		}
		sessions = append(sessions, sessionFromFields(sids[i], fields))
	}
	return sessions, nil
}

func (r *repo) Extend(ctx context.Context, sid string, ttl time.Duration) error {
	key := sessionKey(sid)
	pipe := r.client.TxPipeline()
	pipe.Expire(ctx, key, ttl)
	pipe.HSet(ctx, key, "last_active", time.Now().Unix())
	// 同端互斥键同步续期（其 TTL 与会话一致）
	if uid, device, ok := r.deviceOf(ctx, sid); ok {
		pipe.Expire(ctx, deviceKey(uid, device), ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *repo) Touch(ctx context.Context, sid string) error {
	return r.client.HSet(ctx, sessionKey(sid), "last_active", time.Now().Unix()).Err()
}

// TouchIfDue 节流刷新：SET NX 占住节流位（TTL=interval），占住才真正刷新最近活跃；
// 节流状态存 Redis，多实例共享同一节流窗口
func (r *repo) TouchIfDue(ctx context.Context, sid string, interval time.Duration) error {
	due, err := r.client.SetNX(ctx, touchKey(sid), 1, interval).Result()
	if err != nil {
		return err
	}
	if !due {
		return nil
	}
	return r.Touch(ctx, sid)
}

func (r *repo) Revoke(ctx context.Context, sid string) error {
	s, err := r.Find(ctx, sid)
	if err != nil {
		if err == bizsession.ErrSessionNotFound {
			return nil // 已不存在视为吊销成功（幂等）
		}
		return err
	}
	pipe := r.client.TxPipeline()
	pipe.Del(ctx, sessionKey(sid))
	pipe.Del(ctx, touchKey(sid))
	pipe.SRem(ctx, keySessionIndex, sid)
	pipe.SRem(ctx, userIndexKey(s.UserID), sid)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	// 互斥键仅当仍指向本会话时删除（Lua 原子比较，防误删顶替后的新键）
	_ = r.client.Eval(ctx, luaReleaseDevice, []string{deviceKey(s.UserID, s.Device)}, sid).Err()
	return nil
}

func (r *repo) FindSidsByUser(ctx context.Context, userID uint) ([]string, error) {
	return r.client.SMembers(ctx, userIndexKey(userID)).Result()
}

// deviceOf 读会话归属（uid + 设备端），会话不存在时 ok=false
func (r *repo) deviceOf(ctx context.Context, sid string) (uid uint, device string, ok bool) {
	vals, err := r.client.HMGet(ctx, sessionKey(sid), "uid", "device").Result()
	if err != nil || len(vals) != 2 {
		return 0, "", false
	}
	uidStr, _ := vals[0].(string)
	device, _ = vals[1].(string)
	if uidStr == "" || device == "" {
		return 0, "", false
	}
	return mustUint(uidStr), device, true
}

func sessionFromFields(sid string, fields map[string]string) *bizsession.Session {
	s := &bizsession.Session{ID: sid}
	s.UserID = mustUint(fields["uid"])
	s.Username = fields["username"]
	s.Nickname = fields["nickname"]
	s.Device = bizsession.NormalizeDevice(fields["device"])
	s.IP = fields["ip"]
	s.UserAgent = fields["ua"]
	s.LoginAt = unixToTime(fields["login_at"])
	s.LastActiveAt = unixToTime(fields["last_active"])
	return s
}

func mustUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 64)
	return uint(v)
}

func unixToTime(s string) time.Time {
	v, _ := strconv.ParseInt(s, 10, 64)
	if v <= 0 {
		return time.Time{}
	}
	return time.Unix(v, 0)
}
