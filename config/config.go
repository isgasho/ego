package config

import (
	"github.com/ebar-go/ego/helper"
)

// Config 系统配置项
type Config struct {
	// 服务名称
	ServiceName string

	// 服务端口号
	ServicePort int

	// 响应日志最大长度
	MaxResponseLogSize int

	// 日志路径
	LogPath string

	// jwt的key
	JwtSignKey []byte

	// redis config
	redisConfig *RedisConfig

	// mysql config
	mysqlConfig *MysqlConfig

	// mns config
	mnsConfig *MnsConfig
}

// Redis config
func (config *Config) Redis() *RedisConfig {
	return config.redisConfig
}

// Mysql config
func (config *Config) Mysql() *MysqlConfig {
	return config.mysqlConfig
}

// Mns config
func (config *Config) Mns() *MnsConfig {
	return config.mnsConfig
}

// init 通过读取环境变量初始化系统配置
func NewInstance() *Config {
	instance := &Config{}
	instance.ServiceName = helper.DefaultString(Getenv("SYSTEM_NAME"), "app")
	instance.ServicePort = helper.DefaultInt(helper.String2Int(Getenv("HTTP_PORT")), 8080)

	instance.LogPath = helper.DefaultString(Getenv("LOG_PATH"), "/tmp")
	instance.MaxResponseLogSize = helper.DefaultInt(helper.String2Int(Getenv("MAX_RESPONSE_LOG_SIZE")), 1000)

	instance.JwtSignKey = []byte(Getenv("JWT_KEY"))

	// init mysql config
	instance.redisConfig = &RedisConfig{
		Host: helper.DefaultString(Getenv("REDIS_HOST"), "127.0.0.1"),
		Port: helper.DefaultInt(helper.String2Int(Getenv("REDIS_PORT")), 6379),
		Auth: Getenv("REDIS_AUTH"),
	}
	instance.redisConfig.complete()

	// init redis config
	instance.mysqlConfig = &MysqlConfig{
		Name:     Getenv("MYSQL_DATABASE"),
		Host:     helper.DefaultString(Getenv("MYSQL_MASTER_HOST"), "127.0.0.1"),
		Port:     helper.DefaultInt(helper.String2Int(Getenv("MYSQL_MASTER_PORT")), 3306),
		User:     Getenv("MYSQL_MASTER_USER"),
		Password: Getenv("MYSQL_MASTER_PASS"),
	}
	instance.mysqlConfig.complete()

	// mns config
	instance.mnsConfig = &MnsConfig{
		Url:             Getenv("MNS_ENDPOINT"),
		AccessKeyId:     Getenv("MNS_ACCESSID"),
		AccessKeySecret: Getenv("MNS_ACCESSKEY"),
	}

	return instance
}