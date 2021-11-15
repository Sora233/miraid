package miraid

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/Mrs4s/MiraiGo/binary"
	"github.com/Mrs4s/MiraiGo/client"
	"github.com/Mrs4s/go-cqhttp/coolq"
	"github.com/Mrs4s/go-cqhttp/global"
	_ "github.com/Mrs4s/go-cqhttp/modules/servers"
	"github.com/sirupsen/logrus"
	"github.com/tuotoo/qrcode"
	asc2art "github.com/yinghau76/go-ascii-art"
	"image"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const (
	sessionToken = "session.token"
	deviceJson   = "device.json"
	qrCodePng    = "qrcode.png"
)

var (
	console = bufio.NewReader(os.Stdin)

	readLine = func() (str string) {
		str, _ = console.ReadString('\n')
		str = strings.TrimSpace(str)
		return
	}

	readLineTimeout = func(t time.Duration, defaultV string) (str string) {
		r := make(chan string)
		go func() {
			select {
			case r <- readLine():
			case <-time.After(t):
			}
		}()
		str = defaultV
		select {
		case str = <-r:
		case <-time.After(t):
		}
		return
	}
)

type Bot struct {
	CQBOT  *coolq.CQBot
	config *Config
}

func (b *Bot) Init(config *Config) error {
	if b == nil {
		panic("<nil Bot>")
	}
	b.config = config
	switch config.Method {
	case "miraigo":
		qqclient, err := b.miraiInit()
		if err != nil {
			return err
		}
		b.CQBOT = &coolq.CQBot{Client: qqclient}
	case "go-cqhttp":

	default:
		return ErrUnknownMethod
	}
	return nil
}

func (b *Bot) Run() {

}

func (b *Bot) readToken(sessionFile string) ([]byte, error) {
	return os.ReadFile(sessionFile)
}

func (b *Bot) saveToken() {
	_ = ioutil.WriteFile(sessionToken, b.CQBOT.Client.GenToken(), 0o677)
}

func (b *Bot) clearToken() {
	os.Remove(sessionToken)
}

func (b *Bot) ensureDeviceJson(deviceJson string) error {
	jsonb, err := os.ReadFile(deviceJson)
	if err == nil {
		err = client.SystemDeviceInfo.ReadJson(jsonb)
	}
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Warnf("警告：检测到不存在device信息，将创建于%v，如果是第一次运行，可以忽略", deviceJson)
			client.GenRandomDevice()
			if err := ioutil.WriteFile(deviceJson, client.SystemDeviceInfo.ToJson(), 0755); err != nil {
				return fmt.Errorf("无法创建 %v: %v", deviceJson, err)
			}
		} else {
			return fmt.Errorf("无法读取device信息 %v： %v", deviceJson, err)
		}
	}
	return nil
}

func (b *Bot) miraiInit() (qqClient *client.QQClient, err error) {
	qqClient = client.NewClient(b.config.MiraiGo.Uin, b.config.MiraiGo.Password)
	qqClient.AllowSlider = true

	if err = b.ensureDeviceJson(deviceJson); err != nil {
		return nil, err
	}

	if global.PathExists(sessionToken) {
		var token []byte
		token, err = b.readToken(sessionToken)
		if err != nil {
			goto NormalLogin
		}
		if qqClient.Uin != 0 {
			r := binary.NewReader(token)
			sessionUin := r.ReadInt64()
			if sessionUin != qqClient.Uin {
				logrus.Warnf("QQ号(%v)与会话缓存内的QQ号(%v)不符，将清除会话缓存", qqClient.Uin, sessionUin)
				b.clearToken()
				goto NormalLogin
			}
		}
		if err = qqClient.TokenLogin(token); err != nil {
			b.clearToken()
			qqClient.Disconnect()
			qqClient.Release()
			qqClient = client.NewClientEmpty()
			logrus.Warnf("恢复会话失败: %v , 尝试使用正常流程登录.", err)
			time.Sleep(time.Second)
		} else {
			b.saveToken()
			logrus.Debug("恢复会话成功")
			return
		}
	}
NormalLogin:
	var loginResp *client.LoginResponse
	if qqClient.Uin == 0 {
		logrus.Info("未指定账号密码，请扫码登陆")
		qqClient, err = b.qrcodeLogin(qqClient)
	} else {
		logrus.Info("使用帐号密码登陆")
		loginResp, err = qqClient.Login()
		if err == nil {
			qqClient, err = b.login(qqClient, loginResp)
		}
	}
	if err != nil {
		logrus.Errorf("登陆失败: %v", err)
	} else {
		logrus.Info("登陆成功：%v", qqClient.Nickname)
	}
	return qqClient, err
}

func (b *Bot) qrcodeLogin(qqClient *client.QQClient) (*client.QQClient, error) {
	qrCodeResp, err := qqClient.FetchQRCode()
	if err != nil {
		return nil, err
	}
	fi, err := qrcode.Decode(bytes.NewReader(qrCodeResp.ImageData))
	if err != nil {
		return nil, err
	}
	_ = ioutil.WriteFile(qrCodePng, qrCodeResp.ImageData, 0o644)
	defer func() { _ = os.Remove(qrCodePng) }()
	logrus.Infof("请使用手机QQ扫描二维码 (qrcode.png) : ")
	time.Sleep(time.Second)

	qrcodeTerminal.New().Get(fi.Content).Print()

	status, err := qqClient.QueryQRCodeStatus(qrCodeResp.Sig)
	if err != nil {
		return nil, err
	}
	prevState := status.State
	for {
		time.Sleep(time.Second)
		status, _ = qqClient.QueryQRCodeStatus(qrCodeResp.Sig)
		if status == nil || prevState == status.State {
			continue
		}
		prevState = status.State
		switch status.State {
		case client.QRCodeCanceled:
			logrus.Info("扫码被用户取消")
			return nil, fmt.Errorf("扫码被用户取消")
		case client.QRCodeTimeout:
			logrus.Info("二维码过期")
			return nil, fmt.Errorf("二维码过期")
		case client.QRCodeWaitingForConfirm:
			logrus.Infof("扫码成功, 请在手机端确认登录.")
		case client.QRCodeConfirmed:
			res, err := qqClient.QRCodeLogin(status.LoginInfo)
			if err != nil {
				return nil, err
			}
			return b.login(qqClient, res)
		case client.QRCodeImageFetch, client.QRCodeWaitingForScan:
			// ignore
		}
	}
}

func (b *Bot) login(qqClient *client.QQClient, resp *client.LoginResponse) (*client.QQClient, error) {
	var err error

	for {
		if err != nil {
			return nil, err
		}
		if resp.Success {
			return qqClient, nil
		}

		var text string
		switch resp.Error {
		case client.SliderNeededError:
			// code below copyright by https://github.com/Mrs4s/go-cqhttp
			logrus.Warn("登录需要滑条验证码, 请使用手机QQ扫描二维码以继续登录.")
			qqClient.Disconnect()
			qqClient.Release()
			qqClient = client.NewClientEmpty()
			return b.qrcodeLogin(qqClient)
		case client.NeedCaptcha:
			logrus.Warn("登录需要验证码.")
			img, _, _ := image.Decode(bytes.NewReader(resp.CaptchaImage))
			fmt.Println(asc2art.New("image", img).Art)
			logrus.Warn("请输入验证码 (captcha.jpg)： (Enter 提交)")
			text = readLine()
			resp, err = qqClient.SubmitCaptcha(text, resp.CaptchaSign)
			continue
		case client.SMSNeededError:
			logrus.Warnf("账号已开启设备锁, 按 Enter 向手机 %v 发送短信验证码.", resp.SMSPhone)
			readLine()
			if !qqClient.RequestSMS() {
				logrus.Warnf("发送验证码失败，可能是请求过于频繁.")
				return nil, errors.New("sms send error")
			}
			logrus.Warn("请输入短信验证码： (Enter 提交)")
			text = readLine()
			resp, err = qqClient.SubmitSMS(text)
			continue
		case client.SMSOrVerifyNeededError:
			logrus.Warn("账号已开启设备锁，请选择验证方式:")
			logrus.Warnf("1. 向手机 %v 发送短信验证码", resp.SMSPhone)
			logrus.Warn("2. 使用手机QQ扫码验证.")
			logrus.Warn("请输入(1 - 2) (将在10秒后自动选择2)：")
			text = readLineTimeout(time.Second*10, "2")
			if strings.Contains(text, "1") {
				if !qqClient.RequestSMS() {
					logrus.Warnf("发送验证码失败，可能是请求过于频繁.")
					return nil, errors.New("sms send error")
				}
				logrus.Warn("请输入短信验证码： (Enter 提交)")
				text = readLine()
				resp, err = qqClient.SubmitSMS(text)
				continue
			}
			fallthrough
		case client.UnsafeDeviceError:
			logrus.Warnf("账号已开启设备锁，请前往 -> %v <- 验证后重启Bot.", resp.VerifyUrl)
			logrus.Infof("按 Enter 或等待 5s 后继续....")
			readLineTimeout(time.Second*5, "")
			return nil, fmt.Errorf("登录失败: 账号已开启设备锁")
		case client.OtherLoginError, client.UnknownLoginError, client.TooManySMSRequestError:
			msg := resp.ErrorMessage
			if strings.Contains(msg, "版本") {
				msg = "密码错误或账号被冻结"
			}
			if strings.Contains(msg, "冻结") {
				return nil, errors.New("账号被冻结")
			}
			logrus.Warnf("登录失败: %v", msg)
			logrus.Infof("按 Enter 或等待 5s 后继续....")
			readLineTimeout(time.Second*5, "")
			return nil, fmt.Errorf("登录失败: %v", msg)
		}
	}
}

func (b *Bot) goCQInit() {

}

func NewMiraid() *Bot {
	return new(Bot)
}
