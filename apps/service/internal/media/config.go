package media

import (
	"os"
	"strconv"
	"strings"
)

// Config 描述图片对象存储所需的服务端配置，密钥只从环境变量读取。
type Config struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
	PublicUseSSL   bool
}

// LoadConfig 从环境变量读取 MinIO 配置；配置不完整时保持媒体能力不可用，但不阻止服务启动。
func LoadConfig() Config {
	endpoint := strings.TrimSpace(os.Getenv("MINIO_ENDPOINT"))
	publicEndpoint := strings.TrimSpace(os.Getenv("MINIO_PUBLIC_ENDPOINT"))
	useSSL := readBool("MINIO_USE_SSL")
	publicUseSSL := useSSL
	if publicEndpoint == "" {
		publicEndpoint = endpoint
	}
	if strings.TrimSpace(os.Getenv("MINIO_PUBLIC_USE_SSL")) != "" {
		publicUseSSL = readBool("MINIO_PUBLIC_USE_SSL")
	}
	return Config{
		Endpoint:       endpoint,
		PublicEndpoint: publicEndpoint,
		AccessKey:      strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY")),
		SecretKey:      strings.TrimSpace(os.Getenv("MINIO_SECRET_KEY")),
		Bucket:         strings.TrimSpace(os.Getenv("MINIO_BUCKET")),
		UseSSL:         useSSL,
		PublicUseSSL:   publicUseSSL,
	}
}

// Complete 判断对象存储配置是否包含建立连接所需的全部字段。
func (config Config) Complete() bool {
	return config.Endpoint != "" && config.PublicEndpoint != "" && config.AccessKey != "" && config.SecretKey != "" && config.Bucket != ""
}

// readBool 读取宽松布尔环境变量，未设置或格式错误时按 false 处理。
func readBool(key string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	return err == nil && value
}
