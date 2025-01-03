package Plugins

import (
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shadow1ng/fscan/Common"
	"net"
	"strings"
	"time"
)

// RabbitMQScan 执行 RabbitMQ 服务扫描
func RabbitMQScan(info *Common.HostInfo) (tmperr error) {
	if Common.DisableBrute {
		return
	}

	maxRetries := Common.MaxRetries

	Common.LogDebug(fmt.Sprintf("开始扫描 %v:%v", info.Host, info.Ports))
	Common.LogDebug("尝试默认账号 guest/guest")

	// 先测试默认账号 guest/guest
	user, pass := "guest", "guest"
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		if retryCount > 0 {
			Common.LogDebug(fmt.Sprintf("第%d次重试默认账号: guest/guest", retryCount+1))
		}

		// 执行RabbitMQ连接
		done := make(chan struct {
			success bool
			err     error
		}, 1)

		go func() {
			success, err := RabbitMQConn(info, user, pass)
			select {
			case done <- struct {
				success bool
				err     error
			}{success, err}:
			default:
			}
		}()

		// 等待结果或超时
		var err error
		select {
		case result := <-done:
			err = result.err
			if result.success && err == nil {
				result := fmt.Sprintf("RabbitMQ服务 %v:%v 连接成功 用户名: %v 密码: %v",
					info.Host, info.Ports, user, pass)
				Common.LogSuccess(result)
				return nil
			}
		case <-time.After(time.Duration(Common.Timeout) * time.Second):
			err = fmt.Errorf("连接超时")
		}

		if err != nil {
			errlog := fmt.Sprintf("RabbitMQ服务 %v:%v 尝试失败 用户名: %v 密码: %v 错误: %v",
				info.Host, info.Ports, user, pass, err)
			Common.LogError(errlog)

			if retryErr := Common.CheckErrs(err); retryErr != nil {
				if retryCount == maxRetries-1 {
					continue
				}
				continue
			}
		}
		break
	}

	// 计算总尝试次数
	totalUsers := len(Common.Userdict["rabbitmq"])
	totalPass := len(Common.Passwords)
	total := totalUsers * totalPass
	tried := 0

	Common.LogDebug(fmt.Sprintf("开始尝试用户名密码组合 (总用户数: %d, 总密码数: %d)", totalUsers, totalPass))

	// 遍历其他用户名密码组合
	for _, user := range Common.Userdict["rabbitmq"] {
		for _, pass := range Common.Passwords {
			tried++
			pass = strings.Replace(pass, "{user}", user, -1)
			Common.LogDebug(fmt.Sprintf("[%d/%d] 尝试: %s:%s", tried, total, user, pass))

			// 重试循环
			for retryCount := 0; retryCount < maxRetries; retryCount++ {
				if retryCount > 0 {
					Common.LogDebug(fmt.Sprintf("第%d次重试: %s:%s", retryCount+1, user, pass))
				}

				// 执行RabbitMQ连接
				done := make(chan struct {
					success bool
					err     error
				}, 1)

				go func(user, pass string) {
					success, err := RabbitMQConn(info, user, pass)
					select {
					case done <- struct {
						success bool
						err     error
					}{success, err}:
					default:
					}
				}(user, pass)

				// 等待结果或超时
				var err error
				select {
				case result := <-done:
					err = result.err
					if result.success && err == nil {
						result := fmt.Sprintf("RabbitMQ服务 %v:%v 连接成功 用户名: %v 密码: %v",
							info.Host, info.Ports, user, pass)
						Common.LogSuccess(result)
						return nil
					}
				case <-time.After(time.Duration(Common.Timeout) * time.Second):
					err = fmt.Errorf("连接超时")
				}

				if err != nil {
					errlog := fmt.Sprintf("RabbitMQ服务 %v:%v 尝试失败 用户名: %v 密码: %v 错误: %v",
						info.Host, info.Ports, user, pass, err)
					Common.LogError(errlog)

					if retryErr := Common.CheckErrs(err); retryErr != nil {
						if retryCount == maxRetries-1 {
							continue
						}
						continue
					}
				}
				break
			}
		}
	}

	Common.LogDebug(fmt.Sprintf("扫描完成，共尝试 %d 个组合", tried+1)) // +1 包含默认账号
	return tmperr
}

// RabbitMQConn 尝试 RabbitMQ 连接
func RabbitMQConn(info *Common.HostInfo, user string, pass string) (bool, error) {
	host, port := info.Host, info.Ports
	timeout := time.Duration(Common.Timeout) * time.Second

	// 构造 AMQP URL
	amqpURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, pass, host, port)

	// 配置连接
	config := amqp.Config{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, timeout)
		},
	}

	// 尝试连接
	conn, err := amqp.DialConfig(amqpURL, config)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	// 如果成功连接
	if conn != nil {
		result := fmt.Sprintf("RabbitMQ服务 %v:%v 爆破成功 用户名: %v 密码: %v", host, port, user, pass)
		Common.LogSuccess(result)
		return true, nil
	}

	return false, fmt.Errorf("认证失败")
}
