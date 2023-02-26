package job

import (
	"runtime"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
)

var botInstace *tgbotapi.BotAPI
type LoginStatus byte

const (
	LoginSuccess LoginStatus = 1
	LoginFail    LoginStatus = 0
)

type StatsNotifyJob struct {
	enable         bool
	xrayService    service.XrayService
	inboundService service.InboundService
	settingService service.SettingService
}

func NewStatsNotifyJob() *StatsNotifyJob {
	return new(StatsNotifyJob)
}

func (j *StatsNotifyJob) SendMsgToTgbot(msg string) {
	//Telegram bot basic info
	tgBottoken, err := j.settingService.GetTgBotToken()
	if err != nil || tgBottoken == "" {
		logger.Warning("sendMsgToTgbot failed,GetTgBotToken fail:", err)
		return
	}
	tgBotid, err := j.settingService.GetTgBotChatId()
	if err != nil {
		logger.Warning("sendMsgToTgbot failed,GetTgBotChatId fail:", err)
		return
	}

	bot, err := tgbotapi.NewBotAPI(tgBottoken)
	if err != nil {
		fmt.Println("get tgbot error:", err)
		return
	}
	bot.Debug = true
	fmt.Printf("Authorized on account %s", bot.Self.UserName)
	info := tgbotapi.NewMessage(int64(tgBotid), msg)
	//msg.ReplyToMessageID = int(tgBotid)
	bot.Send(info)
}

func (j *StatsNotifyJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}
	var info string
	//get hostname
	name, err := os.Hostname()
	if err != nil {
		fmt.Println("get hostname error:", err)
		return
	}

	info = fmt.Sprintf("نام سرور : %s\r\n", name)
	//get ip address
	var ip string
	netInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("net.Interfaces failed, err:", err.Error())
		return
	}

	for i := 0; i < len(netInterfaces); i++ {
		if (netInterfaces[i].Flags & net.FlagUp) != 0 {
			addrs, _ := netInterfaces[i].Addrs()

			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						ip = ipnet.IP.String()
						break
					} else {
						ip = ipnet.IP.String()
						break
					}
				}
			}
		}
	}
	info += fmt.Sprintf("آدرس : %s\r\n \r\n", ip)

	//get traffic
	inbouds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("StatsNotifyJob run failed:", err)
		return
	}

	
	for _, inbound := range inbouds {
		info += fmt.Sprintf("✅نام کانفیگ: %s\r\n💡پورت: %d\r\n🔼آپلود↑: %s\r\n🔽دانلود↓: %s\r\n🔄حجم کل:% s\r\n", inbound.Remark, inbound.Port, common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down), common.FormatTraffic((inbound.Up + inbound.Down)))
		if inbound.ExpiryTime == 0 {
			info += fmt.Sprintf("📅تاریخ انقضاء: نامحدود\r\n \r\n")
		} else {
			info += fmt.Sprintf("📅تاریخ انقضاء: %s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
	}
	tgBottoken, err := j.settingService.GetTgBotToken()
	bot, err := tgbotapi.NewBotAPI(tgBottoken)
	tgBotChatId, err := j.settingService.GetTgBotChatId()
	if err != nil {
		logger.Warning("failed ", err)
	}
	msg := tgbotapi.NewDocument(tgBotChatId, FilePath("/etc/x-ui/x-ui.db"))
	bot.Send(msg)
	j.SendMsgToTgbot(info)
}

func (j *StatsNotifyJob) UserLoginNotify(username string, ip string, time string, status LoginStatus) {
	if username == "" || ip == "" || time == "" {
		logger.Warning("UserLoginNotify failed,invalid info")
		return
	}
	var msg string
	//get hostname
	name, err := os.Hostname()
	if err != nil {
		fmt.Println("get hostname error:", err)
		return
	}
	if status == LoginSuccess {
		msg = fmt.Sprintf("با موفقیت به پنل وارد شدید \r\n سرور : %s\r\n", name)
	} else if status == LoginFail {
		msg = fmt.Sprintf("ورود به پنل ناموفق بود \r\n سرور : %s\r\n", name)
	}
	msg += fmt.Sprintf("زمان: %s\r\n", time)
	msg += fmt.Sprintf("نام کاربری: %s\r\n", username)
	msg += fmt.Sprintf("آدرس: %s\r\n", ip)
	j.SendMsgToTgbot(msg)
}

var numericKeyboard = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
        tgbotapi.NewInlineKeyboardButtonData("usage", "get_usage"),
		tgbotapi.NewInlineKeyboardButtonData("delete", "get_delete"),
        tgbotapi.NewInlineKeyboardButtonData("disable", "get_disable"),
        tgbotapi.NewInlineKeyboardButtonData("enable", "get_enable"),
        tgbotapi.NewInlineKeyboardButtonData("clear", "get_clear"),
        tgbotapi.NewInlineKeyboardButtonData("clear all", "clearall"),
        tgbotapi.NewInlineKeyboardButtonData("restart XRAY", "restart"),
        tgbotapi.NewInlineKeyboardButtonData("github", "github"),
		tgbotapi.NewInlineKeyboardButtonData("help", "help"),
    ),
)

func (j *StatsNotifyJob) OnReceive() *StatsNotifyJob {
	tgBottoken, err := j.settingService.GetTgBotToken()
	if err != nil || tgBottoken == "" {
		logger.Warning("sendMsgToTgbot failed,GetTgBotToken fail:", err)
		return j
	}
	bot, err := tgbotapi.NewBotAPI(tgBottoken)
	if err != nil {
		fmt.Println("get tgbot error:", err)
		return j
	}
	bot.Debug = false
	u := tgbotapi.NewUpdate(0)
    u.Timeout = 10

    updates := bot.GetUpdatesChan(u)

    for update := range updates {
        if update.Message == nil { 
			
			if update.CallbackQuery != nil {
				// Respond to the callback query, telling Telegram to show the user
				// a message with the data received.
				callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
				if _, err := bot.Request(callback); err != nil {
					logger.Warning(err)
				}
	
				// And finally, send a message containing the data received.
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")

				switch update.CallbackQuery.Data {
					case "get_usage":
						msg.Text = "برای استفاده شما دستوری مانند این ارسال کنید : \n <code>/usage uuid | id</code> \n مثال : <code>/usage fc3239ed-8f3b-4151-ff51-b183d5182142</code>"
						msg.ParseMode = "HTML"
					case "get_delete":
						msg.Text = "برای حذف کانفیگ از دستور پیروی کنید : \n <code>/delete port</code> \n مثال : <code>/delete 1234</code>"
						msg.ParseMode = "HTML"
					case "get_disable":
						msg.Text = "برای غیرفعال کردن کانفیگ از دستور پیروی کنید : \n <code>/disable port</code> \n مثال : <code>/disable 1234</code>"
						msg.ParseMode = "HTML"
					case "get_enable":
						msg.Text = "برای فعال کردن کانفیگ از دستور پیروی کنید : \n <code>/enable port</code> \n مثال : <code>/enable 1234</code>"
						msg.ParseMode = "HTML"
					case "get_clear":
						msg.Text = "برای ریست ترافیک کانفیگ از دستور پیروی کنید : \n <code>/clear port</code> \n مثال : <code>/clear 1234</code>"
						msg.ParseMode = "HTML"
					}
				if _, err := bot.Send(msg); err != nil {
					logger.Warning(err)
				}
			}
		
            continue
        }

        if !update.Message.IsCommand() {
            continue
        }

        // Create a new MessageConfig. We don't have text yet,
        // so we leave it empty.
        msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

        // Extract the command from the Message.
        switch update.Message.Command() {
	
	case "delete":
		inboundPortStr := update.Message.CommandArguments()
		inboundPortValue, err := strconv.Atoi(inboundPortStr)

		if err != nil {
			msg.Text = "🔴 پورت ورودی نامعتبر است، لطفا بررسی کنید"
			break
		}

		//logger.Infof("Will delete port:%d inbound", inboundPortValue)
		error := j.inboundService.DelInboundByPort(inboundPortValue)
		if error != nil {
			msg.Text = fmt.Sprintf("⚠ حذف کانفیگ به پورت %d انجام نشد", inboundPortValue)
		} else {
			msg.Text = fmt.Sprintf("✅ ورودی پورت با موفقیت حذف شد", inboundPortValue)
		}
		msg.ReplyMarkup = numericKeyboard

	case "restart":
		err := j.xrayService.RestartXray(true)
		if err != nil {
			msg.Text = fmt.Sprintln("⚠ راه اندازی مجدد سرویس XRAY ناموفق بود, err: ", err)
		} else {
			msg.Text = "✅ سرویس XRAY با موفقیت راه اندازی مجدد شد"
		}
		msg.ReplyMarkup = numericKeyboard

	case "disable":
		inboundPortStr := update.Message.CommandArguments()
		inboundPortValue, err := strconv.Atoi(inboundPortStr)
		if err != nil {
			msg.Text = "🔴 پورت ورودی نامعتبر است، لطفا بررسی کنید"
		break
		}
		//logger.Infof("Will delete port:%d inbound", inboundPortValue)
		error := j.inboundService.DisableInboundByPort(inboundPortValue)
		if error != nil {
			msg.Text = fmt.Sprintf("⚠ کانفیگ با پورت %d غیرفعال نشد, err: %s", inboundPortValue, error)
		} else {
			msg.Text = fmt.Sprintf("✅ کانفیگ با پورت %d با موفقیت غیرفعال شد", inboundPortValue)
		}
		msg.ReplyMarkup = numericKeyboard

	case "enable":
		inboundPortStr := update.Message.CommandArguments()
		inboundPortValue, err := strconv.Atoi(inboundPortStr)
		if err != nil {
			msg.Text = "🔴 پورت ورودی نامعتبر است، لطفا بررسی کنید"
			break
		}
		//logger.Infof("Will delete port:%d inbound", inboundPortValue)
		error := j.inboundService.EnableInboundByPort(inboundPortValue)
		if error != nil {
			msg.Text = fmt.Sprintf("⚠ فعال کردن کانفیگ با پورت %d موفق نبود, err: %s", inboundPortValue, error)
		} else {
			msg.Text = fmt.Sprintf("✅ کانفیگ با پورت %d با موفقیت فعال شد ", inboundPortValue)
		}
		msg.ReplyMarkup = numericKeyboard

	case "clear":
		inboundPortStr := update.Message.CommandArguments()
		inboundPortValue, err := strconv.Atoi(inboundPortStr)
		if err != nil {
			msg.Text = "🔴 پورت ورودی نامعتبر است، لطفا بررسی کنید"
			break
		}
		error := j.inboundService.ClearTrafficByPort(inboundPortValue)
		if error != nil {
			msg.Text = fmt.Sprintf("⚠ ریست ترافیک پورت %d انجام نشد, err: %s", inboundPortValue, error)
		} else {
			msg.Text = fmt.Sprintf("✅ ریست ترافیک پورت %d با موفقیت انجام شد", inboundPortValue)
		}
		msg.ReplyMarkup = numericKeyboard

	case "clearall":
		error := j.inboundService.ClearAllInboundTraffic()
		if error != nil {
			msg.Text = fmt.Sprintf("⚠ ریست ترافیک کل کانفیگ ها انجام نشد, err: %s", error)
		} else {
			msg.Text = fmt.Sprintf("✅ تمام ترافیک کانفیگ ها با موفقیت پاکسازی شد")
		}
		msg.ReplyMarkup = numericKeyboard

    case "help":
		msg.Text = "از دکمه های زیر استفاده کنید"
		msg.ReplyMarkup = numericKeyboard

	case "github":
		msg.Text = `💻 لینک پروژه: https://github.com/MrCenTury/xXx-UI/`
		msg.ReplyMarkup = numericKeyboard

	case "status":
		msg.Text = j.GetsystemStatus()
		msg.ReplyMarkup = numericKeyboard

	case "start":
		msg.Text = `
		😁 سلام
		💖به ربات تلگرام پنل xXx-UI خوش آمدید
		👦🏻 Author   : MrCenTury
		📍 Github   : https://github.com/MrCenTury
		📞 Telegram : @hcentury`
		msg.ReplyMarkup = numericKeyboard
		
	case "usage":
		msg.Text = j.getClientUsage(update.Message.CommandArguments())
	
	case "author":msg.Text = `
	👦🏻 Author   : MrCenTury
	📍 Github   : https://github.com/MrCenTury
	📞 Telegram : @hcentury`
        default:
        msg.Text = "⭐/help⭐"
		msg.ReplyMarkup = numericKeyboard

        }

        if _, err := bot.Send(msg); err != nil {
            logger.Warning(err)
        }
    }
	return j
}

func (j*StatsNotifyJob) GetsystemStatus() string {
	var status string
	// get hostname
	name, err := os.Hostname()
	if err != nil {
		fmt.Println("get hostname error: ", err)
		return ""
	}

	status = fmt.Sprintf("😊 Host Name: %s\r\n", name)
	status += fmt.Sprintf("🔗 System: %s\r\n", runtime.GOOS)
	status += fmt.Sprintf("⬛ CPU Load: %s\r\n", runtime.GOARCH)

	avgState, err := load.Avg()
	if err != nil {
		logger.Warning("get load avg failed: ", err)
	} else {
		status += fmt.Sprintf("⭕ System load: %.2f, %.2f, %.2f\r\n", avgState.Load1, avgState.Load5, avgState.Load15)
	}

	upTime, err := host.Uptime()
	if err != nil {
		logger.Warning("get uptime failed: ", err)
	} else {
		status += fmt.Sprintf("⏳ Operation hours: %s\r\n", common.FormatTime(upTime))
	}

	// xray version
	status += fmt.Sprintf("🟡 Current XRay kernel version: %s\r\n", j.xrayService.GetXrayVersion())

	// ip address
	var ip string
	ip = common.GetMyIpAddr()
	status += fmt.Sprintf("🆔 IP Address: %s\r\n \r\n", ip)

	// get traffic
	inbouds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("StatsNotifyJob run error: ", err)
	}

	for _, inbound := range inbouds {
		status += fmt.Sprintf("😎 Inbound remark: %s\r\nport: %d\r\nUplink Traffic↑: %s\r\nDownlink Traffic↓: %s\r\nTotal traffic: %s\r\n", inbound.Remark, inbound.Port, common.FormatTraffic(inbound.Up), common.FormatTraffic(inbound.Down), common.FormatTraffic((inbound.Up + inbound.Down)))
		if inbound.ExpiryTime == 0 {
			status += fmt.Sprintf("⌚ Understanding time: indefinitely\r\n \r\n")
		} else {
			status += fmt.Sprintf("❗ Expire date: %s\r\n \r\n", time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
	}
	return status
}

func (j *StatsNotifyJob) getClientUsage(id string) string {
	traffic , err := j.inboundService.GetClientTrafficById(id)
	if err != nil {
		logger.Warning(err)
		return "something wrong!"
	}
	expiryTime := ""
	if traffic.ExpiryTime == 0 {
		expiryTime = fmt.Sprintf("نامحدود")
	} else {
		expiryTime = fmt.Sprintf("%s", time.Unix((traffic.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
	}
	total := ""
	if traffic.Total == 0 {
		total = fmt.Sprintf("نامحدود")
	} else {
		total = fmt.Sprintf("%s", common.FormatTraffic((traffic.Total)))
	}
	output := fmt.Sprintf("💡 فعال: %t\r\n📧 یوزر: %s\r\n🔼 آپلود↑: %s\r\n🔽 دانلود↓: %s\r\n🔄 حجم کل: %s / %s\r\n📅 انقضاء: %s\r\n",
	traffic.Enable, traffic.Email, common.FormatTraffic(traffic.Up), common.FormatTraffic(traffic.Down), common.FormatTraffic((traffic.Up + traffic.Down)),
	total, expiryTime)
	
	return output
}
