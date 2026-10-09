<!-- last-updated: 2026-10-09 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 跨仓并行会话：grep 空结果先核 cwd 与绝对路径，勿轻断「文件缺失」

## 情境

权限码目录动态化变更（2026-10-09）：平台仓（../embodied-platform）同时有另一 AI 会话在途写代码（dataset 移除 + 数据映射开放面），本会话按 mtime 探测避让后进场补 sdk 改动。`cd sdk && go build` 报 `undefined: TenantUser`，随后多次 `grep "TenantUser" sdk/types.go` 均无结果，据此误判「sdk 类型从未落盘、需从本仓镜像补齐」；实际是 shell 每条命令后 cwd 被重置回本仓、命令内 `cd` 后下一条相对路径 grep 落在错误目录（`sdk/sdk/types.go` 不存在），且系统 grep 为 ugrep——对不存在的文件仅打 warning 不报错，`| head` 下 warning 被吞。types.go 里类型其实一直都在，`go build` 报错是并行会话的瞬时中间态。

## 坑 / 模式

三个因素叠加：①跨仓多会话并行时文件状态本身在变，「build 报错 + grep 空」很容易被解读成断头代码；②本环境 shell cwd 每命令重置，命令内 `cd` 的效果不跨命令，相对路径 grep 会静默查错地方；③ugrep 对不存在文件不报错只 warning。若按误判动手「补类型」会写入重复定义，把可自愈的中间态变成真冲突。**判定另一会话代码缺失/断头前：用绝对路径重新 grep（不依赖 cwd）+ `ls -lT` 核对文件 mtime + 隔几分钟重跑 build 观察是否自愈**；只有三者一致指向缺失才动手补。

## 出现次数

1（首次：2026-10-09 权限目录动态化，平台仓 sdk 判断）

## 状态

pending

## 晋升去向

（暂未晋升；若再次发生跨仓并行误判则晋升至 aiDoc/relations/ 跨仓协作小节）

## 晋升后复发

无（0）

## 记录日期

2026-10-09
