package conf

import (
	"os"
	"testing"
)

func TestEnvOverride(t *testing.T) {
	os.Setenv("APP_SERVER_MODE", "release")
	os.Setenv("APP_DB_MYSQL_HOST", "mysql")
	os.Setenv("APP_DB_MYSQL_PORT", "3307")
	os.Setenv("APP_JWT_SECRET", "env-secret")
	os.Setenv("APP_REDIS_ADDR", "redis:6379")
	defer func() {
		os.Unsetenv("APP_SERVER_MODE")
		os.Unsetenv("APP_DB_MYSQL_HOST")
		os.Unsetenv("APP_DB_MYSQL_PORT")
		os.Unsetenv("APP_JWT_SECRET")
		os.Unsetenv("APP_REDIS_ADDR")
	}()
	c, err := Load("../../configs/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Mode != "release" || c.DB.MySQL.Host != "mysql" || c.DB.MySQL.Port != 3307 || c.JWT.Secret != "env-secret" || c.Redis.Addr != "redis:6379" {
		t.Fatalf("env override not applied: mode=%q mysqlHost=%q mysqlPort=%d jwt=%q redis=%q",
			c.Server.Mode, c.DB.MySQL.Host, c.DB.MySQL.Port, c.JWT.Secret, c.Redis.Addr)
	}
}
