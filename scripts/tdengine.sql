CREATE DATABASE IF NOT EXISTS mxm; -- 创建数据库

-- 创建超级表location,对应sg的location推送
CREATE STABLE mxm.stb_location( `time` timestamp, lati float, `long` float, `type` varchar(12), satellite int, WIFI varchar(1024),  `address` NCHAR(48)) 
    TAGS(deviceType varchar(16), activationDate timestamp);

-- 创建超级表, 对应btt 心跳包

-- CREATE STABLE IF NOT EXISTS mxm.stb_btt_hb(`_ts` timestamp, `csq` VARCHAR(4), `lng` VARCHAR(16), `lat` VARCHAR(16), `alt` VARCHAR(10), 
-- `speed` VARCHAR(12), `direc` varchar(4), `bssid` VARCHAR(255), `rssi` VARCHAR(255), `sates` VARCHAR(4), `snr` VARCHAR(255), `time` timestamp, 
-- `type` VARCHAR(2), `vol` VARCHAR(6), `charging` BOOL, `temp` INT, `gpstime` varchar(12), `step` varchar(12)) TAGS(deviceType varchar(16), activationDate timestamp);

CREATE STABLE IF NOT EXISTS mxm.stb_btt_hb(`_ts` timestamp, msg VARBINARY(2048), res VARBINARY(1024)) TAGS(`type` varchar(8), activDate timestamp)
