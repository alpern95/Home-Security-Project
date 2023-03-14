PROJECT = "gpiodemo"
VERSION = "1.0.0"

--  doit ajouter sys.lua !!!!
local sys = require "sys"
require("sysplus")


log.info("main", "iotda demo")

-- ajout local button
if wdt then
    --添加硬狗防止程序卡死，在支持的设备上启用这个功能
    wdt.init(9000)--初始化watchdog设置为9s
    sys.timerLoopStart(wdt.feed, 3000)--3s喂一次狗
end

local button_timer_outtime = 10 --按键定时器: 10ms
local button_shake_time = 1     --按键消抖时间: button_shake_time*button_timer_outtime
local button_long_time = 100    --按键长按时间: button_shake_time*button_timer_outtime

local button_detect = true
local button_state = false
local button_cont = 0

--local BTN_PIN = 9
local BTN_PIN = 9 -- correspond au bouton BOOT

-- 
if gpio.debounce then
    gpio.debounce(BTN_PIN, 5) -- debounce egal anti rebond de 5 ms
end

button = gpio.setup(BTN_PIN, function() 
        if not button_detect then return end
        button_detect = false
        button_state = true
    end, 
    gpio.PULLUP,
    gpio.FALLING)

button_timer = sys.timerLoopStart(function()
    if button_state then
        if button() == 0 then
            button_cont = button_cont + 1
            if button_cont > button_long_time then
                print("long pass")
            end
        else 
            if button_cont < button_shake_time then
            else
                if button_cont < button_long_time then
                    print("pass")
                else
				    local payload1 = "000"
                    print("long pass")
                end
            end
            button_cont = 0
            button_state = false
            button_detect = true
        end
    end
end,button_timer_outtime) 

-- fin ajout local button
	

local device_id     = "porte-entree"    --Passez à votre propre appareil id
--local device_id     = "apernelle"    --Passez à votre propre appareil id
local device_secret = "*******"    --Changer pour votre propre clé d'appareil

local mqttc = nil

-- gpio12/gpio13
local LEDA= gpio.setup(12, 0, gpio.PULLUP) -- gpio.setup(pin, mode, pull, irq)
local LEDB= gpio.setup(13, 0, gpio.PULLUP)

sys.subscribe("IP_READY", function()
    log.info("mobile", "IP_READY")
	LEDB(1)
end)

sys.subscribe("WLAN_READY", function ()
    if wlan.ready() == 0 then
            log.info("network", "tsend complete, sleep 5s")
			LEDB(1)
        else
            log.warn("main", "wlan is not ready yet")
        	LEDB(0)			
        end	
end)

sys.subscribe("NTP_UPDATE", function()
    log.info("date", "time_ici", os.date())
end)


sys.subscribe("NTP_SYNC_DONE", function()
        log.info("ntp", "done")
        log.info("date", os.date())
    end
)

sys.taskInit(function()
    log.info("wlan", "wlan_init:", wlan.init())
    wlan.setMode(wlan.STATION)
    --wlan.connect("Freebox", "**********************", 1)
	wlan.connect("ZTE_", "", 1)
    local result, data = sys.waitUntil("IP_READY")
    log.info("wlan", "IP_READY", result, data)
	--socket.ntpSync() -- Ajout pour test (ne marche pas)
    --ntp.settz("CET") -- Ajout pour test (ne marche pas)
	--ntp.init("ntp.ntsc.ac.cn")
    local client_id,user_name,password = iotauth.iotda(device_id,device_secret)
    log.info("iotda",client_id,user_name,password)
    mqttc = mqtt.create(nil,"192.168.0.140", 1883)
    mqttc:auth(client_id,user_name,password)
    mqttc:keepalive(30) -- 默认值240s
    mqttc:autoreconn(true, 3000) -- 自动重连机制

    mqttc:on(function(mqtt_client, event, data, payload)
        -- 用户自定义代码
        log.info("mqtt", "event", event, mqtt_client, data, payload)
        if event == "conack" then
            sys.publish("mqtt_conack")
            --mqtt_client:subscribe("/luatos/123456")
			mqtt_client:subscribe("/entree/000001")
			mqtt_client:subscribe("/entree/000002")
			
        elseif event == "recv" then
            log.info("mqtt", "downlink", "topic", data, "payload", payload)
        elseif event == "sent" then
            log.info("mqtt", "sent", "pkgid", data)
        end
    end)

    mqttc:connect()
    sys.wait(10000)
    --mqttc:subscribe("/luatos/123456")
	mqttc:subscribe("/entree/000001")
	mqttc:subscribe("/entree/000002")
	sys.waitUntil("mqtt_conack")
    while true do
        -- mqttc自动处理重连
        local ret, topic, data, qos = sys.waitUntil("mqtt_pub", 30000)
        if ret then
            if topic == "close" then break end
            mqttc:publish(topic, data, qos)
        end
    end
    mqttc:close()
    mqttc = nil
end)

sys.taskInit(function()
	--local topic = "/luatos/123456"
	local topic = "/entree/000001"
	local payload = "0"
	local qos = 1
	--ajout second topic
	local topic1 = "/entree/000002"
	local payload1 = "0"
	local qos1 = 1	
    local result, data = sys.waitUntil("IP_READY")
    while true do
        sys.wait(5000)
        if mqttc:ready() then
            local pkgid = mqttc:publish(topic, payload, qos)
			local pkgid = mqttc:publish(topic1, payload1, qos1)
        end
    end
end)

-- 用户代码已结束---------------------------------------------
-- 结尾总是这一句
sys.run()
-- sys.run()之后后面不要加任何语句!!!!!
