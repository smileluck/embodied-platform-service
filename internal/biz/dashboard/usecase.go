package dashboard

import (
	"context"
	"time"
)

// Usecase 仪表盘聚合用例
type Usecase struct {
	repo Repo
}

func NewUsecase(repo Repo) *Usecase {
	return &Usecase{repo: repo}
}

// Stats 聚合首页数据：趋势按日补零；Redis 故障时在线数返回 0（旁路数据不阻断）
func (uc *Usecase) Stats(ctx context.Context) (*Stats, error) {
	out := &Stats{}
	users, roles, err := uc.repo.Counts()
	if err != nil {
		return nil, err
	}
	out.Cards.Users, out.Cards.Roles = users, roles
	if out.Cards.TodayLogins, err = uc.repo.TodayLogins(); err != nil {
		return nil, err
	}

	login, err := uc.repo.LoginTrend(7)
	if err != nil {
		return nil, err
	}
	op, err := uc.repo.OpTrend(7)
	if err != nil {
		return nil, err
	}
	out.LoginTrend = fillZero(login, 7)
	out.OpTrend = fillZero(op, 7)

	if out.RecentLogins, err = uc.repo.RecentLogins(8); err != nil {
		return nil, err
	}
	return out, nil
}

// fillZero 趋势补零（缺失日期填 0，按日期升序）
func fillZero(in []DailyPoint, days int) []DailyPoint {
	byDate := make(map[string]DailyPoint, len(in))
	for _, p := range in {
		byDate[p.Date] = p
	}
	out := make([]DailyPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := byDate[d]; ok {
			out = append(out, p)
		} else {
			out = append(out, DailyPoint{Date: d})
		}
	}
	return out
}
