create database mxm;
use mxm;

-- user表
 CREATE TABLE `users` (
  `id` int NOT NULL AUTO_INCREMENT,
  `openid` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `nick_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `avatar_url` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `enroll_admin` tinyint(1) DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_openid` (`openid`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=34 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

 CREATE TABLE `devices` (
  `id` char(16) NOT NULL,
  `originSN` varchar(16) NOT NULL,
  `type` varchar(12) NOT NULL,
  `enable` tinyint(1) NOT NULL DEFAULT '0',
  `user_id` int DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `bind_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `label` varchar(26) DEFAULT NULL,
  `last_online` timestamp NULL DEFAULT NULL,
  `last_locate` timestamp NULL DEFAULT NULL,
  `status` varchar(16) NOT NULL DEFAULT '0',
  `profile` json DEFAULT NULL,
  `electricity` int NOT NULL DEFAULT '0',
  `location` json DEFAULT NULL,
  `interval` int NOT NULL DEFAULT '10',
  `sim_card_signal` int NOT NULL DEFAULT '0',
  `steps` int NOT NULL DEFAULT '0',
  `address` varchar(255) DEFAULT NULL,
  `longitude` double DEFAULT NULL,
  `latitude` double DEFAULT NULL,
  `altitude` double DEFAULT NULL,
  `satellites` int DEFAULT NULL,
  `loc_type` varchar(20) DEFAULT NULL,
  `loc_time` timestamp DEFAULT NULL,
  `accuracy` double DEFAULT NULL,
  `speed` double DEFAULT NULL,
  `heading` double DEFAULT NULL,
  `age` int DEFAULT NULL,
  `avatar_url` varchar(255) DEFAULT NULL,
  `description` text,
  `phone_number` varchar(20) DEFAULT NULL,
  `sex` varchar(10) DEFAULT NULL,
  `weight` int DEFAULT NULL,
  `charging` tinyint(1) DEFAULT '0',
  `buzzer` tinyint(1) DEFAULT '0',
  `note` varchar(256) DEFAULT NULL,
  `species` int DEFAULT '1',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_unique_originSN_and_type` (`originSN`,`type`),
  KEY `idx_userId` (`user_id`),
  KEY `idx_deleted_at` (`deleted_at`),
  CONSTRAINT `devices_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `chk_enable` CHECK ((`enable` in (0,1)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 步数统计表
CREATE TABLE device_steps_template (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  steps int NOT NULL  DEFAULT 0
)

-- 设备历史数据模板表（时间序列存储原始数据）
CREATE TABLE device_his_data_template (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    raw_data JSON NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 位置轨迹专用表（优化空间查询）
CREATE TABLE device_his_pos_template (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    `type` varchar(8) NOT NULL,
    loc_time TIMESTAMP NOT NULL,
    latitude DECIMAL(10, 6) NOT NULL,   
    longitude DECIMAL(10, 6) NOT NULL,  
    `address` VARCHAR(255),
    satellites INT,
    accuracy FLOAT,                      
    altitude FLOAT,                      
    speed FLOAT,                         
    heading FLOAT,                       
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

insert into devices (id,originSN,type) 
values("869861062618140","869861062618140","btt");
insert into devices (id,originSN,type) 
values("863644076531434","863644076531434","btt");
insert into devices (id,originSN,type,user_id) 
values("863644076550376","863644076550376","btt",1);//客户机器
insert into devices (id,originSN,type) 
values("863644076550378","863644076550378","btt");//客户机器
insert into devices (id,originSN,type) 
values("863844076531095","863844076531095","btt");//客户机器
insert into devices (id,originSN,type) 
values("863644076543074","863644076543074","btt");//客户机器
insert into devices (id,originSN,type) 
values("863644076531855","863644076531855","btt");//客户机器，四川，姚姚爸，丢失后找回
insert into devices (id,originSN,type) 
values("863644076574525","863644076574525","btt");//lsb机器
insert into devices (id,originSN,type) 
values("868909071389418","868909071389418","btt");//客户机器
insert into devices (id,originSN,type) 
values("860678073623292","860678073623292","btt");//客户机器
insert into devices(id,originSN,type) 
values("863644076547737","863644076547737","btt");//客户机器

// 历史数据分析
select created_at, raw_data->>'$.data.GNSS[0].time' as tm, raw_data->>'$.dataType' as dataType, raw_data->> '$.messageId' as msgID, raw_data->> '$.data.BAT.vol' as vol, raw_data->>'$.data.LTE.csq' as csq  from his_data_860678073623292;



CREATE TABLE `share_mapping` (
  `device_id` CHAR(36) NOT NULL,
  `user_id` INT NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` TIMESTAMP NULL DEFAULT NULL,
  PRIMARY KEY (`device_id`, `user_id`),
  FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (`device_id`) REFERENCES `devices`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `safe_region` (
  `device_id` CHAR(36) NOT NULL,
  `name`      varchar(32),
  `type`      CHAR(12) NOT NULL,
  `area`      VARCHAR(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `recovery_cmds` (
  `id`        int AUTO_INCREMENT PRIMARY KEY,
  `device_id` CHAR(36) NOT NULL,
  `tm`       TIMESTAMP NOT NULL,
  `action`    VARCHAR(256) NOT NULL,
  `args`      VARCHAR(256) 
)

CREATE TABLE `alarms` (
  `id`    int AUTO_INCREMENT PRIMARY KEY,
  `time`  TIMESTAMP,
  `device_id` CHAR(36) NOT NULL REFERENCES `devices`(`id`) ON DELETE CASCADE,
  `type` int NOT NULL, 
  `msg` VARCHAR(32)
)

CREATE TABLE feedback (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    contact VARCHAR(100),
    reply TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE `orders` (
  `id` int NOT NULL AUTO_INCREMENT COMMENT '订单ID', 
  `order_no` varchar(32) NOT NULL COMMENT '商户订单号',
  `description` varchar(255) NOT NULL COMMENT '订单描述',
  `amount` int NOT NULL COMMENT '订单金额(单位:分)',
  `currency` varchar(3) NOT NULL DEFAULT 'CNY' COMMENT '货币类型',
  `openid` varchar(32) DEFAULT NULL COMMENT '用户openid',
  `transaction_id` varchar(32) DEFAULT NULL COMMENT '微信支付订单号',
  `status` enum('created','paid','refunding','refunded','closed','failed') NOT NULL DEFAULT 'created' COMMENT '订单状态',
  `attach` varchar(255) DEFAULT NULL COMMENT '附加数据',
  `prepay_id` varchar(64) DEFAULT NULL COMMENT '预支付交易会话标识',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `paid_at` datetime DEFAULT NULL COMMENT '支付成功时间',
  `expire_time` datetime DEFAULT NULL COMMENT '订单过期时间',
  `refund_amount` int DEFAULT '0' COMMENT '已退款金额(单位:分)',
  `user_id` int COMMENT '关联用户ID',
  `device_info` varchar(32) DEFAULT NULL COMMENT '设备信息',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_order_no` (`order_no`),
  KEY `idx_openid` (`openid`),
  KEY `idx_transaction_id` (`transaction_id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='微信支付订单表';


CREATE TABLE device_settings (
    device_id CHAR(36) PRIMARY KEY,
    auto_start_at VARCHAR(16) NULL,
    auto_start_enable  tinyint(1) NOT NULL DEFAULT '0',
    auto_shut_at VARCHAR(16) NULL,
    auto_shut_enable  tinyint(1) NOT NULL DEFAULT '0',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
