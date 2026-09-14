//go:build wireinject

// wire 依赖注入装配（与 Kratos 相同的 DI 方式，切换时 Provider 图可直接复用）
package main

import (
	"github.com/google/wire"
	base64Captcha "github.com/mojocn/base64Captcha"
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	bizcaptcha "github.com/smilex/smilex-admin-gin/internal/biz/captcha"
	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	bizmerchant "github.com/smilex/smilex-admin-gin/internal/biz/merchant"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	bizrole "github.com/smilex/smilex-admin-gin/internal/biz/role"
	bizsession "github.com/smilex/smilex-admin-gin/internal/biz/session"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	bizuser "github.com/smilex/smilex-admin-gin/internal/biz/user"
	"github.com/smilex/smilex-admin-gin/internal/data"
	dataappuser "github.com/smilex/smilex-admin-gin/internal/data/appuser"
	datablacklist "github.com/smilex/smilex-admin-gin/internal/data/blacklist"
	datacaptcha "github.com/smilex/smilex-admin-gin/internal/data/captcha"
	dataexport "github.com/smilex/smilex-admin-gin/internal/data/export"
	datafile "github.com/smilex/smilex-admin-gin/internal/data/file"
	datalog "github.com/smilex/smilex-admin-gin/internal/data/log"
	datamerchant "github.com/smilex/smilex-admin-gin/internal/data/merchant"
	dataperm "github.com/smilex/smilex-admin-gin/internal/data/permission"
	datarole "github.com/smilex/smilex-admin-gin/internal/data/role"
	datasession "github.com/smilex/smilex-admin-gin/internal/data/session"
	datatenant "github.com/smilex/smilex-admin-gin/internal/data/tenant"
	datauser "github.com/smilex/smilex-admin-gin/internal/data/user"
	"github.com/smilex/smilex-admin-gin/internal/server"
	appusersvc "github.com/smilex/smilex-admin-gin/internal/service/appuser"
	authsvc "github.com/smilex/smilex-admin-gin/internal/service/auth"
	blacklistsvc "github.com/smilex/smilex-admin-gin/internal/service/blacklist"
	exportsvc "github.com/smilex/smilex-admin-gin/internal/service/export"
	filesvc "github.com/smilex/smilex-admin-gin/internal/service/file"
	logsvc "github.com/smilex/smilex-admin-gin/internal/service/log"
	merchantsvc "github.com/smilex/smilex-admin-gin/internal/service/merchant"
	permsvc "github.com/smilex/smilex-admin-gin/internal/service/permission"
	rolesvc "github.com/smilex/smilex-admin-gin/internal/service/role"
	sessionsvc "github.com/smilex/smilex-admin-gin/internal/service/session"
	tenantsvc "github.com/smilex/smilex-admin-gin/internal/service/tenant"
	usersvc "github.com/smilex/smilex-admin-gin/internal/service/user"
)

var bizSet = wire.NewSet(
	bizuser.NewUsecase,
	bizrole.NewUsecase,
	bizperm.NewUsecase,
	bizcaptcha.NewUsecase,
	bizsession.NewUsecase,
	bizlog.NewUsecase,
	bizfile.NewUsecase,
	bizblacklist.NewUsecase,
	bizmerchant.NewUsecase,
	biztenant.NewUsecase,
	bizappuser.NewUsecase,
	bizexport.NewUsecase,
	bizexport.NewRegistry,
	bizexport.NewUserExporter,
	bizexport.NewLoginLogExporter,
	bizexport.NewOpLogExporter,
	auth.NewUsecase,
	// 跨上下文最小依赖接口绑定（provider 与 bind 需同 set）
	wire.Bind(new(auth.CaptchaVerifier), new(*bizcaptcha.Usecase)),
	wire.Bind(new(auth.SessionManager), new(*bizsession.Usecase)),
	wire.Bind(new(bizuser.SessionRevoker), new(*bizsession.Usecase)),
)

var dataRepoSet = wire.NewSet(
	data.NewData,
	data.NewRedisClient,
	data.NewJWTIssuer,
	data.NewAppTokenIssuer,
	datauser.NewRepo,
	datarole.NewRepo,
	dataperm.NewRepo,
	datasession.NewRepo,
	datalog.NewRepo,
	datafile.NewRepo,
	datafile.NewStorageManager,
	datablacklist.NewRepo,
	datamerchant.NewRepo,
	datamerchant.NewAPILogRepo,
	datatenant.NewRepo,
	dataappuser.NewRepo,
	datacaptcha.NewStore,
	dataexport.NewRepo,
	dataexport.NewWorker,
	// 跨上下文最小依赖接口绑定
	wire.Bind(new(base64Captcha.Store), new(*datacaptcha.Store)),
	wire.Bind(new(auth.UserStore), new(bizuser.Repo)),
	wire.Bind(new(auth.RoleNameReader), new(bizrole.Repo)),
	wire.Bind(new(auth.PermissionReader), new(bizperm.Repo)),
	wire.Bind(new(bizlog.Repo), new(*datalog.Repo)),
	wire.Bind(new(bizblacklist.Repo), new(*datablacklist.Repo)),
	wire.Bind(new(bizblacklist.LoginProtector), new(*datablacklist.Repo)),
	wire.Bind(new(bizexport.Enqueuer), new(*dataexport.Worker)),
)

var serviceSet = wire.NewSet(
	authsvc.NewService,
	usersvc.NewService,
	rolesvc.NewService,
	permsvc.NewService,
	sessionsvc.NewService,
	logsvc.NewService,
	filesvc.NewService,
	blacklistsvc.NewService,
	exportsvc.NewService,
	merchantsvc.NewService,
	tenantsvc.NewService,
	appusersvc.NewService,
)

var providerSet = wire.NewSet(bizSet, dataRepoSet, serviceSet, ProvideConfig, server.NewHTTPServer)

// wireApp 由 wire 生成
func wireApp() (*server.HTTPServer, func(), error) {
	wire.Build(providerSet)
	return nil, nil, nil
}
