//go:build wireinject

// wire 依赖注入装配（与 Kratos 相同的 DI 方式，切换时 Provider 图可直接复用）
package main

import (
	"github.com/google/wire"
	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	bizagent "github.com/smilex/smilex-admin-gin/internal/biz/agent"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	bizdevice "github.com/smilex/smilex-admin-gin/internal/biz/device"
	bizdevmodel "github.com/smilex/smilex-admin-gin/internal/biz/devmodel"
	bizdash "github.com/smilex/smilex-admin-gin/internal/biz/dashboard"
	bizdict "github.com/smilex/smilex-admin-gin/internal/biz/dict"
	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	bizjob "github.com/smilex/smilex-admin-gin/internal/biz/job"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	biznotice "github.com/smilex/smilex-admin-gin/internal/biz/notice"
	bizsys "github.com/smilex/smilex-admin-gin/internal/biz/sysconfig"

	bizmonitor "github.com/smilex/smilex-admin-gin/internal/biz/monitor"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	bizrole "github.com/smilex/smilex-admin-gin/internal/biz/role"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/internal/data"
	dataadmission "github.com/smilex/smilex-admin-gin/internal/data/admission"
	dataagent "github.com/smilex/smilex-admin-gin/internal/data/agent"

	dataappuser "github.com/smilex/smilex-admin-gin/internal/data/appuser"
	dataauth "github.com/smilex/smilex-admin-gin/internal/data/auth"
	datablacklist "github.com/smilex/smilex-admin-gin/internal/data/blacklist"
	datadevice "github.com/smilex/smilex-admin-gin/internal/data/device"
	datadevmodel "github.com/smilex/smilex-admin-gin/internal/data/devmodel"
	datadash "github.com/smilex/smilex-admin-gin/internal/data/dashboard"
	datadict "github.com/smilex/smilex-admin-gin/internal/data/dict"
	dataexport "github.com/smilex/smilex-admin-gin/internal/data/export"
	datafile "github.com/smilex/smilex-admin-gin/internal/data/file"
	datajob "github.com/smilex/smilex-admin-gin/internal/data/job"
	datalog "github.com/smilex/smilex-admin-gin/internal/data/log"

	datamonitor "github.com/smilex/smilex-admin-gin/internal/data/monitor"
	datanotice "github.com/smilex/smilex-admin-gin/internal/data/notice"
	dataperm "github.com/smilex/smilex-admin-gin/internal/data/permission"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
	datarole "github.com/smilex/smilex-admin-gin/internal/data/role"

	datasys "github.com/smilex/smilex-admin-gin/internal/data/sysconfig"
	datatenant "github.com/smilex/smilex-admin-gin/internal/data/tenant"
	"github.com/smilex/smilex-admin-gin/internal/server"
	admissionsvc "github.com/smilex/smilex-admin-gin/internal/service/admission"
	agentsvc "github.com/smilex/smilex-admin-gin/internal/service/agent"

	appusersvc "github.com/smilex/smilex-admin-gin/internal/service/appuser"
	authsvc "github.com/smilex/smilex-admin-gin/internal/service/auth"
	blacklistsvc "github.com/smilex/smilex-admin-gin/internal/service/blacklist"
	devicesvc "github.com/smilex/smilex-admin-gin/internal/service/device"
	devmodelsvc "github.com/smilex/smilex-admin-gin/internal/service/devmodel"
	dashsvc "github.com/smilex/smilex-admin-gin/internal/service/dashboard"
	dictsvc "github.com/smilex/smilex-admin-gin/internal/service/dict"
	exportsvc "github.com/smilex/smilex-admin-gin/internal/service/export"
	filesvc "github.com/smilex/smilex-admin-gin/internal/service/file"
	jobsvc "github.com/smilex/smilex-admin-gin/internal/service/job"
	logsvc "github.com/smilex/smilex-admin-gin/internal/service/log"
	noticesvc "github.com/smilex/smilex-admin-gin/internal/service/notice"
	syssvc "github.com/smilex/smilex-admin-gin/internal/service/sysconfig"

	monitorsvc "github.com/smilex/smilex-admin-gin/internal/service/monitor"
	permsvc "github.com/smilex/smilex-admin-gin/internal/service/permission"
	rolesvc "github.com/smilex/smilex-admin-gin/internal/service/role"

	tenantsvc "github.com/smilex/smilex-admin-gin/internal/service/tenant"
)

var bizSet = wire.NewSet(
	bizadmission.NewUsecase,
	bizrole.NewUsecase,
	bizperm.NewUsecase,
	bizlog.NewUsecase,
	bizfile.NewUsecase,
	bizblacklist.NewUsecase,
	biztenant.NewUsecase,
	bizappuser.NewUsecase,
	bizdevice.NewUsecase,
	bizdevmodel.NewUsecase,
	bizdict.NewUsecase,
	bizdash.NewUsecase,
	bizsys.NewUsecase,
	biznotice.NewUsecase,
	bizjob.NewUsecase,
	bizmonitor.NewUsecase,
	bizagent.NewUsecase,
	bizexport.NewUsecase,
	bizexport.NewRegistry,
	bizexport.NewUserExporter,
	bizexport.NewOpLogExporter,
	auth.NewUsecase,
	// 跨上下文最小依赖接口绑定（provider 与 bind 需同 set）
	wire.Bind(new(auth.AdmissionReader), new(*bizadmission.Usecase)),
	// 设备注册自愈：平台 403 时经租户用例补链重建（LinkOrCreate 幂等）
	wire.Bind(new(bizdevice.TenantRelinker), new(*biztenant.Usecase)),
	// 智能体内置只读工具：服务器状态查询复用 monitor 用例
	wire.Bind(new(bizagent.ServerStatusReader), new(*bizmonitor.Usecase)),
)

var dataRepoSet = wire.NewSet(
	data.NewData,
	data.NewRedisClient,
	data.NewAppTokenIssuer,
	data.NewRBACCache,
	dataadmission.NewRepo,
	datarole.NewRepo,
	dataperm.NewRepo,
	datalog.NewRepo,
	datafile.NewRepo,
	datafile.NewStorageManager,
	datablacklist.NewRepo,
	datatenant.NewRepo,
	dataappuser.NewRepo,

	dataagent.NewRepo,

	datadict.NewRepo,

	datadash.NewRepo,
	datamonitor.NewSnapshotRepo,
	wire.Bind(new(bizmonitor.SnapshotRepo), new(*datamonitor.SnapshotRepo)),
	datasys.NewRepo,

	datanotice.NewRepo,
	dataexport.NewRepo,
	dataexport.NewWorker,
	// 平台集成层（商户 HMAC 开放面 + 身份自省 + 存储）
	platform.NewIdentityClient,
	platform.NewStorageClient,
	platform.NewOpenAPIClient,
	platform.NewMerchantIdentity,
	// 平台网关适配器
	datadevice.NewGatewayAdapter,
	datadevmodel.NewGatewayAdapter,
	datatenant.NewSyncer,
	datatenant.NewTenantAvailability,
	dataadmission.NewPlatformGateway,
	dataauth.NewIdentityAdapter,
	// 跨上下文最小依赖接口绑定（provider 与 bind 需同 set）
	wire.Bind(new(auth.IdentitySource), new(*dataauth.IdentityAdapter)),
	datajob.NewRepo,
	wire.Bind(new(bizjob.LogCleaner), new(*datalog.Repo)),
	wire.Bind(new(bizjob.ExportCleaner), new(*dataexport.Worker)),
	wire.Bind(new(bizagent.Repo), new(*dataagent.Repo)),
	wire.Bind(new(bizjob.UsageCleaner), new(*dataagent.Repo)),
	wire.Bind(new(auth.RoleNameReader), new(bizrole.Repo)),
	wire.Bind(new(auth.PermissionReader), new(bizperm.Repo)),
	wire.Bind(new(bizlog.Repo), new(*datalog.Repo)),
	wire.Bind(new(bizblacklist.Repo), new(*datablacklist.Repo)),
	wire.Bind(new(bizblacklist.LoginProtector), new(*datablacklist.Repo)),
	wire.Bind(new(bizexport.Enqueuer), new(*dataexport.Worker)),
	wire.Bind(new(biztenant.PlatformSyncer), new(*datatenant.Syncer)),
	wire.Bind(new(bizdevice.Gateway), new(*datadevice.GatewayAdapter)),
	wire.Bind(new(bizdevmodel.Gateway), new(*datadevmodel.GatewayAdapter)),
	wire.Bind(new(bizadmission.MemberGateway), new(*dataadmission.PlatformGateway)),
	wire.Bind(new(bizadmission.MerchantIdentity), new(*platform.MerchantIdentity)),
	wire.Bind(new(bizadmission.DecisionCache), new(*data.RBACCache)),
	wire.Bind(new(bizrole.DecisionCache), new(*data.RBACCache)),
)

var serviceSet = wire.NewSet(
	authsvc.NewService,
	admissionsvc.NewService,
	rolesvc.NewService,
	permsvc.NewService,
	logsvc.NewService,
	filesvc.NewService,
	blacklistsvc.NewService,
	exportsvc.NewService,
	tenantsvc.NewService,
	appusersvc.NewService,
	devicesvc.NewService,
	devmodelsvc.NewService,
	monitorsvc.NewService,
	agentsvc.NewService,
	dictsvc.NewService,
	dashsvc.NewService,
	syssvc.NewService,
	noticesvc.NewService,
	jobsvc.NewService,
)

var providerSet = wire.NewSet(bizSet, dataRepoSet, serviceSet, ProvideConfig, server.NewHTTPServer)

// wireApp 由 wire 生成
func wireApp() (*server.HTTPServer, func(), error) {
	wire.Build(providerSet)
	return nil, nil, nil
}
