-- 诗境 · MySQL 初始化脚本
-- 说明：后端启动会用 GORM AutoMigrate 自动建表（AUTO_MIGRATE=true）。
-- 本文件用于「手动初始化 / 微信云托管 MySQL 控制台导入 / 结构参考」，与模型保持一致。
-- 字符集统一 utf8mb4；JSON 内容字段用 longtext 存（与 GORM serializer:json 兼容）。

CREATE DATABASE IF NOT EXISTS `shijing` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
USE `shijing`;

-- 用户
CREATE TABLE IF NOT EXISTS `users` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `open_id`        VARCHAR(64)  NULL,
  `session_token`  VARCHAR(64)  NULL,
  `nickname`       VARCHAR(64)  NULL,
  `avatar_char`    VARCHAR(8)   NULL,
  `avatar_color`   VARCHAR(96)  NULL,
  `avatar_url`     VARCHAR(512) NULL,
  `is_admin`       TINYINT(1)   NULL DEFAULT 0,
  `exp`            BIGINT       NULL DEFAULT 0,
  `read_seconds`   BIGINT       NULL DEFAULT 0,
  `member_until`   DATETIME(3)  NULL,
  `member_claimed` TINYINT(1)   NULL DEFAULT 0,
  `created_at`     DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_users_open_id` (`open_id`),
  KEY `idx_users_session_token` (`session_token`),
  KEY `idx_users_is_admin` (`is_admin`),
  KEY `idx_users_exp` (`exp`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 诗词（读/境/意/悟 富内容；JSON 字段用 longtext）
CREATE TABLE IF NOT EXISTS `poems` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `title`            VARCHAR(64)  NULL,
  `dynasty`          VARCHAR(16)  NULL,
  `author`           VARCHAR(32)  NULL,
  `form`             VARCHAR(32)  NULL,
  `year_label`       VARCHAR(64)  NULL,
  `first_verse`      VARCHAR(128) NULL,
  `audio_url`        VARCHAR(512) NULL,
  `illust_url`       VARCHAR(512) NULL,
  `tags`             LONGTEXT     NULL,
  `moods`            LONGTEXT     NULL,
  `translation`      LONGTEXT     NULL,
  `trans_source`     VARCHAR(255) NULL,
  `scene_title`      VARCHAR(64)  NULL,
  `scene_sub`        VARCHAR(64)  NULL,
  `scene_rows`       LONGTEXT     NULL,
  `scene_punch`      TEXT         NULL,
  `scene_background` TEXT         NULL,
  `emotions`         LONGTEXT     NULL,
  `life_curve`       LONGTEXT     NULL,
  `intent_lead`      TEXT         NULL,
  `intent_sub`       TEXT         NULL,
  `reasons`          LONGTEXT     NULL,
  `strategies`       LONGTEXT     NULL,
  `audiences`        LONGTEXT     NULL,
  `void`             TEXT         NULL,
  `layers`           LONGTEXT     NULL,
  `quotes`           LONGTEXT     NULL,
  `duels`            LONGTEXT     NULL,
  `ai_prompts`       LONGTEXT     NULL,
  `comment_count`    BIGINT       NULL DEFAULT 0,
  `like_count`       BIGINT       NULL DEFAULT 0,
  `favorite_count`   BIGINT       NULL DEFAULT 0,
  `status`           VARCHAR(16)  NULL DEFAULT 'published',
  `created_at`       DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  KEY `idx_poems_dynasty` (`dynasty`),
  KEY `idx_poems_author` (`author`),
  KEY `idx_poems_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 逐句原文 + 注释
CREATE TABLE IF NOT EXISTS `poem_lines` (
  `id`        BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `poem_id`   BIGINT UNSIGNED NULL,
  `seq`       BIGINT       NULL,
  `text`      VARCHAR(255) NULL,
  `note_body` TEXT         NULL,
  `note_src`  VARCHAR(255) NULL,
  `rubies`    LONGTEXT     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_poem_lines_poem_id` (`poem_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 评论 / 诗话（parent_id>0 为楼中楼回复）
CREATE TABLE IF NOT EXISTS `comments` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `poem_id`       BIGINT UNSIGNED NULL,
  `parent_id`     BIGINT UNSIGNED NULL DEFAULT 0,
  `reply_to_name` VARCHAR(64)  NULL,
  `user_id`       BIGINT UNSIGNED NULL,
  `author_name`   VARCHAR(64)  NULL,
  `author_badge`  VARCHAR(32)  NULL,
  `avatar_char`   VARCHAR(8)   NULL,
  `avatar_color`  VARCHAR(96)  NULL,
  `content`       TEXT         NULL,
  `kind`          VARCHAR(16)  NULL,
  `like_count`    BIGINT       NULL DEFAULT 0,
  `reply_count`   BIGINT       NULL DEFAULT 0,
  `is_featured`   TINYINT(1)   NULL DEFAULT 0,
  `is_pinned`     TINYINT(1)   NULL DEFAULT 0,
  `pinned_at`     DATETIME(3)  NULL,
  `created_at`    DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  KEY `idx_comments_poem_id` (`poem_id`),
  KEY `idx_comments_parent_id` (`parent_id`),
  KEY `idx_comments_kind` (`kind`),
  KEY `idx_comments_is_pinned` (`is_pinned`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 评论点赞（一人一赞）
CREATE TABLE IF NOT EXISTS `comment_likes` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `comment_id` BIGINT UNSIGNED NULL,
  `user_id`    BIGINT UNSIGNED NULL,
  `created_at` DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_comment_user` (`comment_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 诗词收藏（一人一诗）
CREATE TABLE IF NOT EXISTS `favorites` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NULL,
  `poem_id`    BIGINT UNSIGNED NULL,
  `created_at` DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_poem` (`user_id`, `poem_id`),
  KEY `idx_favorites_poem_id` (`poem_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 诗词点赞（一人一赞）
CREATE TABLE IF NOT EXISTS `poem_likes` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `poem_id`    BIGINT UNSIGNED NULL,
  `user_id`    BIGINT UNSIGNED NULL,
  `created_at` DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_poem_user` (`poem_id`, `user_id`),
  KEY `idx_poem_likes_poem_id` (`poem_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 每日任务（限每类经验的单日获取量，天级重置）
CREATE TABLE IF NOT EXISTS `daily_tasks` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NULL,
  `date`       VARCHAR(10)  NULL,
  `task`       VARCHAR(16)  NULL,
  `count`      BIGINT       NULL DEFAULT 0,
  `exp`        BIGINT       NULL DEFAULT 0,
  `updated_at` DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_date_task` (`user_id`, `date`, `task`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 阅读历史（一人一首，用于首页避开已读）
CREATE TABLE IF NOT EXISTS `read_histories` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NULL,
  `poem_id`    BIGINT UNSIGNED NULL,
  `created_at` DATETIME(3)  NULL,
  `updated_at` DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_read_poem` (`user_id`, `poem_id`),
  KEY `idx_read_histories_poem_id` (`poem_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 非会员与某位诗人剩余的对话次数
CREATE TABLE IF NOT EXISTS `poet_ask_credits` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NULL,
  `poet_key`   VARCHAR(80)  NULL,
  `remain`     BIGINT       NULL DEFAULT 0,
  `updated_at` DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_ask_credit` (`user_id`, `poet_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 看完广告后增加对话次数的记录，用于每日上限
CREATE TABLE IF NOT EXISTS `poet_ask_grants` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NULL,
  `poet_key`   VARCHAR(80)  NULL,
  `day`        VARCHAR(10)  NULL,
  `created_at` DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ask_grant` (`user_id`, `day`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 已有库升级：诗句增加难字注音。可重复执行。
SET @ruby_col := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'poem_lines' AND COLUMN_NAME = 'rubies'
);
SET @ruby_ddl := IF(
  @ruby_col = 0,
  'ALTER TABLE `poem_lines` ADD COLUMN `rubies` LONGTEXT NULL',
  'SELECT 1'
);
PREPARE stmt_ruby FROM @ruby_ddl;
EXECUTE stmt_ruby;
DEALLOCATE PREPARE stmt_ruby;

-- 已有库升级：users 增加登录凭证。可重复执行。
SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'session_token'
);
SET @ddl := IF(
  @col_exists = 0,
  'ALTER TABLE `users` ADD COLUMN `session_token` VARCHAR(64) NULL, ADD KEY `idx_users_session_token` (`session_token`)',
  'SELECT 1'
);
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 运营配置。member_month_price 为会员月价，单位分，600 表示 6 元。改这个值即可调价。
CREATE TABLE IF NOT EXISTS `app_settings` (
  `name`  VARCHAR(64) NOT NULL,
  `value` VARCHAR(64) NULL,
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO `app_settings` (`name`, `value`)
SELECT 'member_month_price', '600'
WHERE NOT EXISTS (SELECT 1 FROM `app_settings` WHERE `name` = 'member_month_price');

-- 虚拟支付配置也可写在这里。道具价格必须与 member_month_price 一致，单位分。
-- INSERT INTO `app_settings` (`name`, `value`) VALUES ('vp_offer_id', '基础配置里的 OfferID');
-- INSERT INTO `app_settings` (`name`, `value`) VALUES ('vp_app_key', '现网 AppKey');
-- INSERT INTO `app_settings` (`name`, `value`) VALUES ('vp_product_id', '会员道具 ID');

-- 会员充值订单。支付成功后才把 status 改为 paid 并延长会员。
CREATE TABLE IF NOT EXISTS `member_orders` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `out_trade_no`   VARCHAR(32)  NULL,
  `user_id`        BIGINT UNSIGNED NULL,
  `open_id`        VARCHAR(64)  NULL,
  `amount`         BIGINT       NULL DEFAULT 0,
  `months`         BIGINT       NULL DEFAULT 1,
  `status`         VARCHAR(16)  NULL,
  `transaction_id` VARCHAR(64)  NULL,
  `created_at`     DATETIME(3)  NULL,
  `paid_at`        DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_member_orders_no` (`out_trade_no`),
  KEY `idx_member_orders_user` (`user_id`),
  KEY `idx_member_orders_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 管理员账号（网页后台登录以此身份鉴权；也可由后端 EnsureAdmin 自动创建）
INSERT INTO `users` (`open_id`, `nickname`, `avatar_char`, `is_admin`, `created_at`)
SELECT 'admin-root', '管理员', '管', 1, NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM `users` WHERE `open_id` = 'admin-root');


