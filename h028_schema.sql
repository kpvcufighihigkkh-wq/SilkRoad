-- Table structure for table `auth_group_permissions`
--

DROP TABLE IF EXISTS `auth_group_permissions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `auth_group_permissions` (
  `id` int NOT NULL AUTO_INCREMENT,
  `group_id` int NOT NULL,
  `permission_id` int NOT NULL,
  `value` tinyint(1) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `permission` (`group_id`,`permission_id`),
  KEY `permission_id` (`permission_id`),
  CONSTRAINT `auth_group_permissions_ibfk_1` FOREIGN KEY (`group_id`) REFERENCES `auth_groups` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `auth_group_permissions_ibfk_2` FOREIGN KEY (`permission_id`) REFERENCES `auth_permissions` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=1221 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `auth_groups`
--

DROP TABLE IF EXISTS `auth_groups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `auth_groups` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=62 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `auth_permissions`
--

DROP TABLE IF EXISTS `auth_permissions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `auth_permissions` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` varchar(64) NOT NULL,
  `name` varchar(64) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB AUTO_INCREMENT=472 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `auth_users`
--

DROP TABLE IF EXISTS `auth_users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `auth_users` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `username` varchar(64) NOT NULL,
  `email` varchar(64) NOT NULL,
  `password` text NOT NULL,
  `group_id` int NOT NULL,
  `created` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `modified` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `username` (`username`),
  UNIQUE KEY `email` (`email`),
  KEY `group_id` (`group_id`),
  CONSTRAINT `auth_users_ibfk_1` FOREIGN KEY (`group_id`) REFERENCES `auth_groups` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=58 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `auth_users_simple`
--

DROP TABLE IF EXISTS `auth_users_simple`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `auth_users_simple` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `username` varchar(64) NOT NULL,
  `email` varchar(64) NOT NULL,
  `password` varchar(32) NOT NULL,
  `group_id` int NOT NULL,
  `created` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `modified` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `username` (`username`),
  UNIQUE KEY `password` (`password`),
  UNIQUE KEY `email` (`email`),
  KEY `group_id` (`group_id`),
  CONSTRAINT `auth_users_simple_ibfk_1` FOREIGN KEY (`group_id`) REFERENCES `auth_groups` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=62 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `auth_users_tokens`
--

DROP TABLE IF EXISTS `auth_users_tokens`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `auth_users_tokens` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `token` text NOT NULL,
  `created` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  CONSTRAINT `auth_users_tokens_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `auth_users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `bobbins`
--

DROP TABLE IF EXISTS `bobbins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `bobbins` (
  `bobbin_id` bigint unsigned NOT NULL,
  `bobbin_id_plc` int DEFAULT NULL,
  `doffing_id` bigint unsigned NOT NULL,
  `place_in_winder` tinyint NOT NULL,
  `plant_area_code` varchar(32) DEFAULT NULL,
  `position_id` int DEFAULT NULL,
  `place` int NOT NULL,
  `sorting_grade_id` int DEFAULT NULL,
  `weight_grade_id` int DEFAULT NULL,
  `final_grade_id` int DEFAULT NULL,
  `defect_id` int DEFAULT NULL,
  `vision_grade_id` int DEFAULT NULL,
  `knitting_grade_id` int DEFAULT NULL,
  `vision_defect_id` int DEFAULT NULL,
  `to_weight` tinyint(1) NOT NULL,
  `weight` decimal(10,3) DEFAULT NULL,
  `created` timestamp NULL DEFAULT NULL,
  `modified` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`bobbin_id`),
  KEY `doffing_id` (`doffing_id`),
  KEY `position_id` (`position_id`),
  KEY `defect_code` (`defect_id`),
  KEY `sorting_grade_id` (`sorting_grade_id`),
  KEY `work_bobbins_ibfk_6` (`weight_grade_id`),
  KEY `work_bobbins_ibfk_7` (`final_grade_id`),
  KEY `bobbins_created_IDX` (`created`) USING BTREE,
  KEY `bobbins_plant_area_code_IDX` (`plant_area_code`) USING BTREE,
  KEY `bobbins_modified_IDX` (`modified`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `box_bobbins`
--

DROP TABLE IF EXISTS `box_bobbins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `box_bobbins` (
  `box_id` int NOT NULL,
  `dty_bobbin_id` int NOT NULL,
  UNIQUE KEY `box_bobbins_un` (`box_id`,`dty_bobbin_id`),
  UNIQUE KEY `box_bobbins_un1` (`dty_bobbin_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `clients_supervision_settings`
--

DROP TABLE IF EXISTS `clients_supervision_settings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `clients_supervision_settings` (
  `ip_address` varchar(100) NOT NULL,
  `settings` longtext,
  PRIMARY KEY (`ip_address`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `defects`
--

DROP TABLE IF EXISTS `defects`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `defects` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` tinyint NOT NULL,
  `name` text NOT NULL,
  `description` text NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB AUTO_INCREMENT=50 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `doffers_alarms`
--

DROP TABLE IF EXISTS `doffers_alarms`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `doffers_alarms` (
  `id` int NOT NULL AUTO_INCREMENT,
  `spinning_side_id` int NOT NULL,
  `alarm_id` int NOT NULL,
  `doffer_number` tinyint NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `doffers_alarms_FK` (`spinning_side_id`),
  KEY `doffers_alarms_doffers_alarms_definition_FK` (`alarm_id`),
  CONSTRAINT `doffers_alarms_doffers_alarms_definition_FK` FOREIGN KEY (`alarm_id`) REFERENCES `doffers_alarms_definition` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `doffers_alarms_FK` FOREIGN KEY (`spinning_side_id`) REFERENCES `spinning_sides` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `doffers_alarms_definition`
--

DROP TABLE IF EXISTS `doffers_alarms_definition`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `doffers_alarms_definition` (
  `id` int NOT NULL AUTO_INCREMENT,
  `word` tinyint NOT NULL,
  `bit` tinyint NOT NULL,
  `name` varchar(100) DEFAULT NULL,
  `chinese_name` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `doffers_alarms_definition_un` (`word`,`bit`)
) ENGINE=InnoDB AUTO_INCREMENT=641 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `doffers_cycles`
--

DROP TABLE IF EXISTS `doffers_cycles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `doffers_cycles` (
  `id` int NOT NULL AUTO_INCREMENT,
  `spinning_side_id` int NOT NULL,
  `doffer_number` int NOT NULL,
  `cycle_id` int NOT NULL,
  `cycle_number` int NOT NULL,
  `cycle_value` int NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `newtable_un` (`spinning_side_id`,`doffer_number`,`cycle_id`,`cycle_number`),
  KEY `newtable_FK` (`cycle_id`),
  CONSTRAINT `newtable_FK` FOREIGN KEY (`cycle_id`) REFERENCES `doffers_cycles_definition` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `newtable_FK_1` FOREIGN KEY (`spinning_side_id`) REFERENCES `spinning_sides` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `doffers_cycles_definition`
--

DROP TABLE IF EXISTS `doffers_cycles_definition`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `doffers_cycles_definition` (
  `id` int NOT NULL,
  `name` varchar(100) DEFAULT NULL,
  `chinese_name` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `doffers_status`
--

DROP TABLE IF EXISTS `doffers_status`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `doffers_status` (
  `id` int NOT NULL AUTO_INCREMENT,
  `spinning_side_id` int NOT NULL,
  `status` tinyint NOT NULL,
  `doffer_number` tinyint NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `doffers_status_FK` (`spinning_side_id`),
  CONSTRAINT `doffers_status_FK` FOREIGN KEY (`spinning_side_id`) REFERENCES `spinning_sides` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `doffings`
--

DROP TABLE IF EXISTS `doffings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `doffings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `winder_id` int DEFAULT NULL,
  `doff_no` int DEFAULT NULL,
  `end_time` datetime DEFAULT NULL,
  `yarn_type` varchar(12) DEFAULT NULL,
  `code_number` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `lot_id` int DEFAULT NULL,
  `team_turn` int DEFAULT NULL,
  `plant_area_code` varchar(32) DEFAULT NULL,
  `created` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `day` date DEFAULT (curdate()),
  `shift_doff_no` int DEFAULT NULL,
  `sent_to_erp` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `doffings_un` (`winder_id`,`doff_no`,`day`,`lot_id`),
  KEY `winder_id` (`winder_id`),
  KEY `lot_id` (`lot_id`),
  KEY `created` (`created`) USING BTREE,
  KEY `doffings_end_time_IDX` (`end_time`) USING BTREE,
  CONSTRAINT `doffings_ibfk_1` FOREIGN KEY (`winder_id`) REFERENCES `winders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `doffings_ibfk_2` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=898775 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `doffings_hourly_production`
--

DROP TABLE IF EXISTS `doffings_hourly_production`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `doffings_hourly_production` (
  `spinning_side_id` int NOT NULL,
  `value` int NOT NULL,
  `day` date NOT NULL,
  `hour` tinyint NOT NULL,
  `timestamp` timestamp NOT NULL,
  PRIMARY KEY (`spinning_side_id`,`day`,`hour`),
  CONSTRAINT `doffings_hourly_production_FK` FOREIGN KEY (`spinning_side_id`) REFERENCES `spinning_sides` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty`
--

DROP TABLE IF EXISTS `dty`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty` (
  `id` int NOT NULL,
  `code` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_bobbins`
--

DROP TABLE IF EXISTS `dty_bobbins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_bobbins` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `dty_order_id` int DEFAULT NULL,
  `dty_box_id` int DEFAULT NULL,
  `plc_id` int DEFAULT NULL,
  `box_of_order` int NOT NULL,
  `bobbins_amount` int NOT NULL,
  PRIMARY KEY (`id`),
  KEY `dty_bobbins_dty_orders_FK` (`dty_order_id`),
  KEY `dty_bobbins_dty_boxes_FK` (`dty_box_id`),
  CONSTRAINT `dty_bobbins_dty_boxes_FK` FOREIGN KEY (`dty_box_id`) REFERENCES `dty_boxes` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `dty_bobbins_dty_orders_FK` FOREIGN KEY (`dty_order_id`) REFERENCES `dty_orders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_boxes`
--

DROP TABLE IF EXISTS `dty_boxes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_boxes` (
  `id` int NOT NULL AUTO_INCREMENT,
  `loading_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `dty_order_id` int DEFAULT NULL,
  `weight` decimal(10,2) DEFAULT NULL,
  `labeling_time` timestamp NULL DEFAULT NULL,
  `box_of_order` int DEFAULT NULL,
  `bobbins_amount` int DEFAULT NULL,
  `daily_id` int DEFAULT NULL,
  `team_turn` int DEFAULT NULL,
  `product_date` date DEFAULT NULL,
  `lot_id` int DEFAULT NULL,
  `palletizer_id` int DEFAULT NULL,
  `order_grade_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `dty_boxes_unique` (`dty_order_id`,`box_of_order`),
  KEY `dty_boxes_FK` (`lot_id`),
  KEY `dty_boxes_FK_1` (`order_grade_id`),
  KEY `dty_boxes_FK_2` (`palletizer_id`),
  CONSTRAINT `dty_boxes_dty_orders_FK` FOREIGN KEY (`dty_order_id`) REFERENCES `dty_orders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `dty_boxes_FK` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `dty_boxes_FK_1` FOREIGN KEY (`order_grade_id`) REFERENCES `order_grades` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `dty_boxes_FK_2` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=49066 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_module_bobbins`
--

DROP TABLE IF EXISTS `dty_module_bobbins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_module_bobbins` (
  `dty_module_id` int NOT NULL,
  `dty_bobbin_id` int NOT NULL,
  UNIQUE KEY `dty_module_bobbins_un` (`dty_module_id`,`dty_bobbin_id`),
  UNIQUE KEY `dty_module_bobbins_un1` (`dty_bobbin_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_modules`
--

DROP TABLE IF EXISTS `dty_modules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_modules` (
  `id` int NOT NULL AUTO_INCREMENT,
  `loading_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `number` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_orders`
--

DROP TABLE IF EXISTS `dty_orders`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_orders` (
  `id` int NOT NULL AUTO_INCREMENT,
  `palletizer_id` int NOT NULL,
  `lot_id` int NOT NULL,
  `order_grade_id` int NOT NULL,
  `bobbins_amount` int NOT NULL,
  `pallets_amount` int NOT NULL,
  `pallet_level` int NOT NULL,
  `operator_number` int NOT NULL,
  `destination` int NOT NULL,
  `type` enum('manual','automatic') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT 'manual',
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `end_time` timestamp NULL DEFAULT NULL,
  `boxes_number` int NOT NULL,
  PRIMARY KEY (`id`),
  KEY `dty_orders_palletizers_FK` (`palletizer_id`),
  KEY `dty_orders_order_grades_FK` (`order_grade_id`),
  KEY `dty_orders_lots_FK` (`lot_id`),
  CONSTRAINT `dty_orders_lots_FK` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `dty_orders_order_grades_FK` FOREIGN KEY (`order_grade_id`) REFERENCES `order_grades` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `dty_orders_palletizers_FK` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=1266 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_pallets`
--

DROP TABLE IF EXISTS `dty_pallets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_pallets` (
  `id` int NOT NULL AUTO_INCREMENT,
  `dty_order_id` int NOT NULL,
  `pallet_of_order` int NOT NULL,
  `number_of_boxes` int DEFAULT NULL,
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `daily_id` int DEFAULT NULL,
  `product_date` date DEFAULT NULL,
  `team_turn` int DEFAULT NULL,
  `rfid` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `bobbins_amount` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `dty_pallets_unique` (`dty_order_id`,`pallet_of_order`),
  KEY `dty_pallets_dty_orders_FK` (`dty_order_id`),
  CONSTRAINT `dty_pallets_dty_orders_FK` FOREIGN KEY (`dty_order_id`) REFERENCES `dty_orders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=2462 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_pallets_boxes`
--

DROP TABLE IF EXISTS `dty_pallets_boxes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_pallets_boxes` (
  `dty_pallet_id` int NOT NULL,
  `dty_box_id` int NOT NULL,
  UNIQUE KEY `dty_pallets_boxes_unique` (`dty_pallet_id`,`dty_box_id`),
  KEY `dty_pallets_boxes_dty_boxes_FK` (`dty_box_id`),
  CONSTRAINT `dty_pallets_boxes_dty_boxes_FK` FOREIGN KEY (`dty_box_id`) REFERENCES `dty_boxes` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `dty_pallets_boxes_dty_pallets_FK` FOREIGN KEY (`dty_pallet_id`) REFERENCES `dty_pallets` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_warehouse_orders`
--

DROP TABLE IF EXISTS `dty_warehouse_orders`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_warehouse_orders` (
  `id` int NOT NULL AUTO_INCREMENT,
  `lot_id` int NOT NULL,
  `number_of_modules` int NOT NULL,
  `modules_sent` int NOT NULL DEFAULT '0',
  `status` enum('to-start','started','completed') NOT NULL DEFAULT 'to-start',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `priority` int DEFAULT NULL,
  `dty_id` int NOT NULL,
  PRIMARY KEY (`id`),
  KEY `knitting_orders_lots_FK` (`lot_id`),
  KEY `knitting_orders_status_IDX` (`status`) USING BTREE,
  KEY `dty_warehouse_orders_dty_FK` (`dty_id`),
  CONSTRAINT `dty_warehouse_orders_dty_FK` FOREIGN KEY (`dty_id`) REFERENCES `dty` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `dty_warehouse_orders_ibfk_1` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=12719 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `dty_warehouse_orders_modules`
--

DROP TABLE IF EXISTS `dty_warehouse_orders_modules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `dty_warehouse_orders_modules` (
  `dty_warehouse_order_id` int NOT NULL,
  `module_number` int NOT NULL,
  `module_id` int DEFAULT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `status` int NOT NULL DEFAULT '0',
  `number_of_bobbins` int DEFAULT NULL,
  `total_bobbins_weight` decimal(10,3) DEFAULT NULL,
  UNIQUE KEY `dty_warehouse_order_id` (`dty_warehouse_order_id`,`module_number`),
  KEY `module_id` (`module_id`),
  CONSTRAINT `dty_warehouse_orders_modules_ibfk_1` FOREIGN KEY (`dty_warehouse_order_id`) REFERENCES `dty_warehouse_orders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `dty_warehouse_orders_modules_ibfk_2` FOREIGN KEY (`module_id`) REFERENCES `modules` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `erp_bobbins`
--

DROP TABLE IF EXISTS `erp_bobbins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `erp_bobbins` (
  `bobbin_id` bigint unsigned NOT NULL,
  `doffing_id` bigint unsigned DEFAULT NULL,
  `doffing_creation_date` datetime DEFAULT NULL,
  `module_number` int DEFAULT NULL,
  `lot_code` varchar(40) DEFAULT NULL,
  `specification` varchar(100) DEFAULT NULL,
  `winder_name` varchar(8) DEFAULT NULL,
  `spinning_line_name` varchar(8) DEFAULT NULL,
  `position` tinyint DEFAULT NULL,
  `grade` tinyint DEFAULT NULL,
  `defect` varchar(100) DEFAULT NULL,
  `weight` decimal(10,3) DEFAULT NULL,
  `write_date` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `sent_to_erp` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`bobbin_id`),
  KEY `erp_bobbins_sent_to_erp_IDX` (`sent_to_erp`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `erp_dty_pallets`
--

DROP TABLE IF EXISTS `erp_dty_pallets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `erp_dty_pallets` (
  `dty_pallet_id` int NOT NULL,
  `request_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `sent_to_erp` tinyint(1) NOT NULL DEFAULT '0',
  `sent_to_warehouse` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`dty_pallet_id`),
  CONSTRAINT `erp_dty_pallets_dty_pallets_FK` FOREIGN KEY (`dty_pallet_id`) REFERENCES `dty_pallets` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `erp_pallets`
--

DROP TABLE IF EXISTS `erp_pallets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `erp_pallets` (
  `pallet_id` int NOT NULL,
  `request_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `sent_to_erp` tinyint(1) NOT NULL DEFAULT '0',
  `sent_to_warehouse` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`pallet_id`),
  CONSTRAINT `erp_pallets_ibfk_1` FOREIGN KEY (`pallet_id`) REFERENCES `pallets` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `final_grades`
--

DROP TABLE IF EXISTS `final_grades`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `final_grades` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` tinyint NOT NULL,
  `name` text NOT NULL,
  `chinese_name` text NOT NULL,
  `description` text NOT NULL,
  `color` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `knitting_grades`
--

DROP TABLE IF EXISTS `knitting_grades`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `knitting_grades` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` tinyint NOT NULL,
  `name` text NOT NULL,
  `chinese_name` text NOT NULL,
  `description` text NOT NULL,
  `color` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `knitting_orders`
--

DROP TABLE IF EXISTS `knitting_orders`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `knitting_orders` (
  `id` int NOT NULL AUTO_INCREMENT,
  `lot_id` int NOT NULL,
  `number_of_modules` int NOT NULL,
  `modules_sent` int NOT NULL DEFAULT '0',
  `knitting_id` int NOT NULL,
  `status` enum('to-start','started','completed') NOT NULL DEFAULT 'to-start',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `priority` int DEFAULT NULL,
  `type` enum('fdy','dty') DEFAULT 'fdy',
  PRIMARY KEY (`id`),
  KEY `knitting_orders_lots_FK` (`lot_id`),
  KEY `knitting_orders_knittings_FK` (`knitting_id`),
  KEY `knitting_orders_status_IDX` (`status`) USING BTREE,
  CONSTRAINT `knitting_orders_knittings_FK` FOREIGN KEY (`knitting_id`) REFERENCES `knittings` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `knitting_orders_lots_FK` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=29865 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `knitting_orders_modules`
--

DROP TABLE IF EXISTS `knitting_orders_modules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `knitting_orders_modules` (
  `knitting_order_id` int NOT NULL,
  `module_number` int NOT NULL,
  `module_id` int DEFAULT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `status` int NOT NULL DEFAULT '0',
  UNIQUE KEY `knitting_orders_modules_unique` (`knitting_order_id`,`module_number`),
  KEY `knitting_orders_modules_modules_FK` (`module_id`),
  KEY `knitting_orders_modules_knitting_orders_FK` (`knitting_order_id`),
  CONSTRAINT `knitting_orders_modules_knitting_orders_FK` FOREIGN KEY (`knitting_order_id`) REFERENCES `knitting_orders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `knitting_orders_modules_modules_FK` FOREIGN KEY (`module_id`) REFERENCES `modules` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `knittings`
--

DROP TABLE IF EXISTS `knittings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `knittings` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` varchar(100) DEFAULT NULL,
  `name` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `licenses`
--

DROP TABLE IF EXISTS `licenses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `licenses` (
  `id` int NOT NULL AUTO_INCREMENT,
  `expiring_date` timestamp NULL DEFAULT NULL,
  `banner_date` timestamp NULL DEFAULT NULL,
  `status` enum('unpaid','paid') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT 'unpaid',
  `password` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `loading_bobbins_sequence`
--

DROP TABLE IF EXISTS `loading_bobbins_sequence`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `loading_bobbins_sequence` (
  `tunnel_side` varchar(3) DEFAULT NULL,
  `spindle_place` int DEFAULT NULL,
  `spindle` enum('A','B') DEFAULT NULL,
  `new_place` int DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `lot_grades_ranges`
--

DROP TABLE IF EXISTS `lot_grades_ranges`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `lot_grades_ranges` (
  `id` int NOT NULL AUTO_INCREMENT,
  `lot_id` int NOT NULL,
  `weight_grade_id` int NOT NULL,
  `min` decimal(10,3) NOT NULL,
  `max` decimal(10,3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `lot_grades_ranges_UN` (`lot_id`,`weight_grade_id`),
  KEY `lot_grades_ranges_FK_1` (`weight_grade_id`),
  CONSTRAINT `lot_grades_ranges_FK` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `lot_grades_ranges_FK_1` FOREIGN KEY (`weight_grade_id`) REFERENCES `weight_grades` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=544 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `lot_prefixes`
--

DROP TABLE IF EXISTS `lot_prefixes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `lot_prefixes` (
  `id` int NOT NULL AUTO_INCREMENT,
  `prefix` varchar(16) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `prefix` (`prefix`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `lot_weights`
--

DROP TABLE IF EXISTS `lot_weights`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `lot_weights` (
  `id` int NOT NULL AUTO_INCREMENT,
  `lot_id` int NOT NULL,
  `level` int NOT NULL,
  `gross` decimal(10,1) NOT NULL,
  `net` decimal(10,1) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `lot_id` (`lot_id`,`level`),
  CONSTRAINT `lot_weights_ibfk_1` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=1850 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `lots`
--

DROP TABLE IF EXISTS `lots`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `lots` (
  `id` int NOT NULL AUTO_INCREMENT,
  `prefix` varchar(64) NOT NULL,
  `code` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `paper_tube_color_id` int DEFAULT NULL,
  `specification_china` varchar(64) NOT NULL,
  `specification_export` varchar(64) DEFAULT NULL,
  `name` varchar(64) NOT NULL,
  `chinese_name` varchar(64) NOT NULL,
  `standard_china` varchar(64) NOT NULL,
  `standard_export` varchar(64) NOT NULL,
  `destination` int NOT NULL DEFAULT '1',
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `modified` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `visible` tinyint(1) NOT NULL DEFAULT '1',
  `type` varchar(100) DEFAULT NULL,
  `order_code` varchar(64) DEFAULT NULL,
  `lustre` varchar(32) DEFAULT NULL,
  `default_pallet_level` int NOT NULL DEFAULT '9',
  `default_pallet_size` int NOT NULL DEFAULT '1200',
  `minimum_pallets_for_order` int NOT NULL DEFAULT '1',
  `default_destination` int NOT NULL DEFAULT '1',
  `maximum_pallets_for_order` int NOT NULL DEFAULT '99',
  `wait_time` int DEFAULT '16',
  `packing_lock` tinyint(1) DEFAULT '0',
  `end_lot` tinyint(1) DEFAULT '0',
  `twist` varchar(24) DEFAULT NULL,
  `box_weight` decimal(10,2) DEFAULT '0.00',
  `order_code_aa1` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `order_code_aa2` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `order_code_a` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `lots_un` (`code`,`order_code`),
  KEY `paper_tube_color_id` (`paper_tube_color_id`),
  CONSTRAINT `lots_ibfk_1` FOREIGN KEY (`paper_tube_color_id`) REFERENCES `paper_tube_colors` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=1513 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `modules`
--

DROP TABLE IF EXISTS `modules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `modules` (
  `id` int NOT NULL AUTO_INCREMENT,
  `number` int NOT NULL,
  `loading_time` datetime DEFAULT CURRENT_TIMESTAMP,
  `doffing1_id` bigint NOT NULL,
  `doffing2_id` bigint NOT NULL,
  `loading_side` enum('sx','dx','sxw','dxw','fdy') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `spinning_side_id` int DEFAULT NULL,
  `sorting_id` int DEFAULT NULL,
  `sorting_start_time` timestamp NULL DEFAULT NULL,
  `sorting_end_time` timestamp NULL DEFAULT NULL,
  `grade_of_module` int DEFAULT NULL,
  `weight_station` int DEFAULT NULL,
  `reprint_time` timestamp NULL DEFAULT NULL,
  `knitting_confirmed` tinyint(1) DEFAULT '0',
  `knitting_order_id` int DEFAULT NULL,
  `knitting_insert_time` timestamp NULL DEFAULT NULL,
  `knitting_result_time` timestamp NULL DEFAULT NULL,
  `sent_to_erp` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `modules_number_IDX` (`number`,`doffing1_id`,`doffing2_id`) USING BTREE,
  KEY `modules_doffing1_id_IDX` (`doffing1_id`) USING BTREE,
  KEY `modules_doffing2_id_IDX` (`doffing2_id`) USING BTREE,
  KEY `modules_FK` (`sorting_id`),
  KEY `modules_FK_1` (`grade_of_module`),
  KEY `modules_knitting_orders_FK` (`knitting_order_id`),
  CONSTRAINT `modules_FK` FOREIGN KEY (`sorting_id`) REFERENCES `sortings` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `modules_FK_1` FOREIGN KEY (`grade_of_module`) REFERENCES `sorting_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `modules_knitting_orders_FK` FOREIGN KEY (`knitting_order_id`) REFERENCES `knitting_orders` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=273685 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `modules_hourly_production`
--

DROP TABLE IF EXISTS `modules_hourly_production`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `modules_hourly_production` (
  `spinning_side_id` int NOT NULL,
  `value` int NOT NULL,
  `day` date NOT NULL,
  `hour` tinyint NOT NULL,
  `timestamp` timestamp NOT NULL,
  PRIMARY KEY (`spinning_side_id`,`day`,`hour`),
  CONSTRAINT `modules_hourly_production_FK` FOREIGN KEY (`spinning_side_id`) REFERENCES `spinning_sides` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `modules_status`
--

DROP TABLE IF EXISTS `modules_status`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `modules_status` (
  `module_number` int NOT NULL,
  `status` tinyint NOT NULL DEFAULT '0',
  `lot_id` int DEFAULT NULL,
  `warehouse_id` int NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `column` int DEFAULT '0',
  `row` int DEFAULT '0',
  `place` int DEFAULT '0',
  `after_knitting_confirm` tinyint(1) NOT NULL DEFAULT '0',
  `to_be_taken` tinyint(1) NOT NULL DEFAULT '0',
  `doffing1_id` int DEFAULT NULL,
  `doffing2_id` int DEFAULT NULL,
  `place_disabled` tinyint(1) NOT NULL DEFAULT '0',
  `monorail_id` int NOT NULL DEFAULT '1',
  `after_knitting` tinyint(1) NOT NULL DEFAULT '0',
  UNIQUE KEY `modules_status_unique` (`module_number`),
  KEY `modules_status_lots_FK` (`lot_id`),
  KEY `modules_status_warehouses_FK` (`warehouse_id`),
  KEY `modules_status_monorails_FK` (`monorail_id`),
  CONSTRAINT `modules_status_lots_FK` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `modules_status_monorails_FK` FOREIGN KEY (`monorail_id`) REFERENCES `monorails` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `modules_status_warehouses_FK` FOREIGN KEY (`warehouse_id`) REFERENCES `warehouses` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `modules_tracking`
--

DROP TABLE IF EXISTS `modules_tracking`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `modules_tracking` (
  `id` int NOT NULL AUTO_INCREMENT,
  `section_id` int NOT NULL,
  `carrier_number` int NOT NULL,
  `module_number` int NOT NULL,
  `lot_code` varchar(100) DEFAULT NULL,
  `doffing_id_1` int DEFAULT NULL,
  `doffing_id_2` int DEFAULT NULL,
  `packaging_destination` int DEFAULT NULL,
  `warehouse_destination` int DEFAULT NULL,
  `sorting_destination` int DEFAULT NULL,
  `spinning_destination` int DEFAULT NULL,
  `destination` int DEFAULT NULL,
  `data` text,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `modules_tracking_carrier_number_IDX` (`carrier_number`) USING BTREE,
  KEY `modules_tracking_module_number_IDX` (`module_number`) USING BTREE,
  KEY `modules_tracking_monorail_sections_FK` (`section_id`),
  CONSTRAINT `modules_tracking_monorail_sections_FK` FOREIGN KEY (`section_id`) REFERENCES `monorail_sections` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `monorail_sections`
--

DROP TABLE IF EXISTS `monorail_sections`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `monorail_sections` (
  `id` int NOT NULL AUTO_INCREMENT,
  `monorail_id` int NOT NULL,
  `section_code` int NOT NULL,
  `name` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `monorail_sections_unique` (`monorail_id`,`section_code`),
  CONSTRAINT `monorail_sections_monorails_FK` FOREIGN KEY (`monorail_id`) REFERENCES `monorails` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `monorails`
--

DROP TABLE IF EXISTS `monorails`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `monorails` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `description` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `settings` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `type` enum('poy','fdy','dty') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT 'poy',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `movements`
--

DROP TABLE IF EXISTS `movements`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `movements` (
  `bobbin_id` bigint unsigned NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `old_position_id` int DEFAULT NULL,
  `new_position_id` int DEFAULT NULL,
  `old_place` int NOT NULL,
  `new_place` int NOT NULL,
  `old_sorting_grade_id` int DEFAULT NULL,
  `new_sorting_grade_id` int DEFAULT NULL,
  `old_weight_grade_id` int DEFAULT NULL,
  `new_weight_grade_id` int DEFAULT NULL,
  `old_final_grade_id` int DEFAULT NULL,
  `new_final_grade_id` int DEFAULT NULL,
  `old_defect_id` int DEFAULT NULL,
  `new_defect_id` int DEFAULT NULL,
  `old_to_weight` tinyint(1) NOT NULL,
  `new_to_weight` tinyint(1) NOT NULL,
  `old_weight` decimal(10,1) DEFAULT NULL,
  `new_weight` decimal(10,1) DEFAULT NULL,
  `old_plant_area_code` varchar(32) DEFAULT NULL,
  `new_plant_area_code` varchar(32) DEFAULT NULL,
  KEY `bobbin_id` (`bobbin_id`),
  KEY `old_position_id` (`old_position_id`),
  KEY `new_position_id` (`new_position_id`),
  KEY `old_sorting_grade_id` (`old_sorting_grade_id`),
  KEY `new_sorting_grade_di` (`new_sorting_grade_id`),
  KEY `old_defect_id` (`old_defect_id`),
  KEY `new_defect_id` (`new_defect_id`),
  KEY `old_weight_grade_id` (`old_weight_grade_id`),
  KEY `new_weight_grade_id` (`new_weight_grade_id`),
  KEY `old_final_grade_id` (`old_final_grade_id`),
  KEY `new_final_grade_id` (`new_final_grade_id`),
  KEY `timestamp` (`timestamp`) USING BTREE,
  CONSTRAINT `movements_ibfk_10` FOREIGN KEY (`old_final_grade_id`) REFERENCES `final_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_11` FOREIGN KEY (`new_final_grade_id`) REFERENCES `final_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_2` FOREIGN KEY (`old_position_id`) REFERENCES `positions` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_3` FOREIGN KEY (`new_position_id`) REFERENCES `positions` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_4` FOREIGN KEY (`old_sorting_grade_id`) REFERENCES `sorting_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_5` FOREIGN KEY (`new_sorting_grade_id`) REFERENCES `sorting_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_6` FOREIGN KEY (`old_defect_id`) REFERENCES `defects` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_7` FOREIGN KEY (`new_defect_id`) REFERENCES `defects` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_8` FOREIGN KEY (`old_weight_grade_id`) REFERENCES `weight_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `movements_ibfk_9` FOREIGN KEY (`new_weight_grade_id`) REFERENCES `weight_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `notifications`
--

DROP TABLE IF EXISTS `notifications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `notifications` (
  `id` int NOT NULL AUTO_INCREMENT,
  `data` varchar(256) DEFAULT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `acknowledge` tinyint(1) NOT NULL DEFAULT '0',
  `type` varchar(100) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `notifications_unique` (`data`)
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `order_grades`
--

DROP TABLE IF EXISTS `order_grades`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `order_grades` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` tinyint NOT NULL,
  `name` text NOT NULL,
  `chinese_name` text NOT NULL,
  `description` text NOT NULL,
  `color` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `orders`
--

DROP TABLE IF EXISTS `orders`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `orders` (
  `id` int NOT NULL AUTO_INCREMENT,
  `palletizer_id` int NOT NULL,
  `lot_id` int NOT NULL,
  `order_grade_id` int NOT NULL,
  `bobbins_amount` int NOT NULL,
  `pallets_amount` int NOT NULL,
  `pallet_level` int NOT NULL,
  `operator_number` int NOT NULL,
  `destination` int NOT NULL,
  `type` enum('manual','automatic') NOT NULL DEFAULT 'manual',
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `end_time` timestamp NULL DEFAULT NULL,
  `pallet_size` int NOT NULL DEFAULT '1200',
  `bobbins_sent` int NOT NULL DEFAULT '0',
  `status` enum('to-start','started','completed') NOT NULL DEFAULT 'to-start',
  `packing_started` tinyint(1) NOT NULL DEFAULT '0',
  `orders_queue_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `orders_ibfk_1` (`lot_id`),
  KEY `orders_ibfk_2` (`order_grade_id`),
  KEY `palletizer_id` (`palletizer_id`),
  KEY `orders_orders_queue_FK` (`orders_queue_id`),
  CONSTRAINT `orders_ibfk_1` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `orders_ibfk_2` FOREIGN KEY (`order_grade_id`) REFERENCES `order_grades` (`id`) ON UPDATE CASCADE,
  CONSTRAINT `orders_ibfk_3` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON UPDATE CASCADE,
  CONSTRAINT `orders_orders_queue_FK` FOREIGN KEY (`orders_queue_id`) REFERENCES `orders_queue` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=33797 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `orders_queue`
--

DROP TABLE IF EXISTS `orders_queue`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `orders_queue` (
  `id` int NOT NULL AUTO_INCREMENT,
  `palletizer_id` int NOT NULL,
  `lot_id` int NOT NULL,
  `order_grade_id` int NOT NULL,
  `bobbins_amount` int NOT NULL,
  `pallets_amount` int NOT NULL,
  `pallet_level` int NOT NULL,
  `operator_number` int NOT NULL,
  `destination` int NOT NULL,
  `type` enum('manual','automatic') NOT NULL DEFAULT 'manual',
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `end_time` timestamp NULL DEFAULT NULL,
  `pallet_size` int DEFAULT '0',
  `bobbins_sent` int NOT NULL DEFAULT '0',
  `status` enum('to-start','started','completed') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT 'to-start',
  `packing_started` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `orders_ibfk_1` (`lot_id`),
  KEY `orders_ibfk_2` (`order_grade_id`),
  KEY `palletizer_id` (`palletizer_id`),
  CONSTRAINT `orders_queue_ibfk_1` FOREIGN KEY (`lot_id`) REFERENCES `lots` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `orders_queue_ibfk_2` FOREIGN KEY (`order_grade_id`) REFERENCES `order_grades` (`id`) ON UPDATE CASCADE,
  CONSTRAINT `orders_queue_ibfk_3` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=24 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `orders_queue_modules`
--

DROP TABLE IF EXISTS `orders_queue_modules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `orders_queue_modules` (
  `module_number` int NOT NULL,
  `orders_queue_id` int DEFAULT NULL,
  `order_id` int DEFAULT NULL,
  KEY `orders_queue_modules_orders_FK` (`order_id`),
  KEY `orders_queue_modules_orders_queue_FK` (`orders_queue_id`),
  CONSTRAINT `orders_queue_modules_orders_FK` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `orders_queue_modules_orders_queue_FK` FOREIGN KEY (`orders_queue_id`) REFERENCES `orders_queue` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `packing_orders_modules`
--

DROP TABLE IF EXISTS `packing_orders_modules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `packing_orders_modules` (
  `packing_order_id` int NOT NULL,
  `module_number` int NOT NULL,
  `module_id` int DEFAULT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `status` int NOT NULL DEFAULT '0',
  `number_of_bobbins` int NOT NULL DEFAULT '0',
  UNIQUE KEY `packing_orders_modules_unique` (`packing_order_id`,`module_number`),
  KEY `packing_orders_modules_modules_FK` (`module_id`),
  CONSTRAINT `packing_orders_modules_modules_FK` FOREIGN KEY (`module_id`) REFERENCES `modules` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `packing_orders_modules_orders_FK` FOREIGN KEY (`packing_order_id`) REFERENCES `orders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `pallet_bobbins`
--

DROP TABLE IF EXISTS `pallet_bobbins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `pallet_bobbins` (
  `pallet_id` int NOT NULL,
  `place` int NOT NULL,
  `winder_name` varchar(8) NOT NULL,
  `spinning_line_name` varchar(8) NOT NULL,
  `place_in_winder` tinyint NOT NULL,
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `bobbin_id` int DEFAULT NULL,
  UNIQUE KEY `place_in_pallet` (`pallet_id`,`place`),
  KEY `pallet_id` (`pallet_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `pallet_boxes`
--

DROP TABLE IF EXISTS `pallet_boxes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `pallet_boxes` (
  `pallet_id` int NOT NULL,
  `box_id` int NOT NULL,
  UNIQUE KEY `pallet_boxes_un` (`pallet_id`,`box_id`),
  UNIQUE KEY `pallet_boxes_un1` (`box_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers`
--

DROP TABLE IF EXISTS `palletizers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` int NOT NULL,
  `name` varchar(64) NOT NULL,
  `description` text NOT NULL,
  `settings` longtext NOT NULL,
  `max_pallets_per_day_shift` int DEFAULT NULL,
  `max_pallets_per_night_shift` int DEFAULT NULL,
  `pallets_record_time` int DEFAULT NULL,
  `last_pallet_timestamp` timestamp NULL DEFAULT NULL,
  `section` varchar(10) DEFAULT NULL,
  `type` enum('poy','fdy','dty') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT 'fdy',
  `monorail_id` int DEFAULT NULL,
  `pallets_size` int NOT NULL DEFAULT '1050',
  PRIMARY KEY (`id`),
  KEY `palletizers_FK` (`monorail_id`),
  CONSTRAINT `palletizers_FK` FOREIGN KEY (`monorail_id`) REFERENCES `monorails` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=COMPACT;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers_alarms`
--

DROP TABLE IF EXISTS `palletizers_alarms`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers_alarms` (
  `id` int NOT NULL AUTO_INCREMENT,
  `palletizer_id` int NOT NULL,
  `alarm_id` int NOT NULL,
  `number` tinyint NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `palletizers_alarms_FK` (`palletizer_id`),
  KEY `palletizers_alarms_FK_1` (`alarm_id`),
  CONSTRAINT `palletizers_alarms_FK` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `palletizers_alarms_FK_1` FOREIGN KEY (`alarm_id`) REFERENCES `palletizers_alarms_definition` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers_alarms_definition`
--

DROP TABLE IF EXISTS `palletizers_alarms_definition`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers_alarms_definition` (
  `id` int NOT NULL AUTO_INCREMENT,
  `word` tinyint NOT NULL,
  `bit` tinyint NOT NULL,
  `name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `chinese_name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `palletizers_alarms_definition_un` (`word`,`bit`)
) ENGINE=InnoDB AUTO_INCREMENT=641 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers_cycles`
--

DROP TABLE IF EXISTS `palletizers_cycles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers_cycles` (
  `id` int NOT NULL AUTO_INCREMENT,
  `number` int DEFAULT NULL,
  `palletizer_id` int NOT NULL,
  `cycle_id` int NOT NULL,
  `cycle_number` int NOT NULL,
  `cycle_value` int NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `palletizers_cycles_UN` (`number`,`palletizer_id`,`cycle_id`,`cycle_number`),
  KEY `palletizers_cycles_FK` (`palletizer_id`),
  KEY `palletizers_cycles_FK_1` (`cycle_id`),
  CONSTRAINT `palletizers_cycles_FK` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `palletizers_cycles_FK_1` FOREIGN KEY (`cycle_id`) REFERENCES `palletizers_cycles_definition` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers_cycles_definition`
--

DROP TABLE IF EXISTS `palletizers_cycles_definition`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers_cycles_definition` (
  `id` int NOT NULL,
  `name` varchar(100) DEFAULT NULL,
  `chinese_name` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers_hourly_production`
--

DROP TABLE IF EXISTS `palletizers_hourly_production`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers_hourly_production` (
  `palletizer_id` int NOT NULL,
  `value` int DEFAULT NULL,
  `day` date NOT NULL,
  `hour` tinyint NOT NULL,
  `timestamp` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`palletizer_id`,`day`,`hour`),
  CONSTRAINT `palletizers_hourly_production_fk` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers_last_modules`
--

DROP TABLE IF EXISTS `palletizers_last_modules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers_last_modules` (
  `palletizer_id` int NOT NULL,
  `module_number` int NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `side` enum('left','right') NOT NULL,
  UNIQUE KEY `palletizers_last_modules_UN` (`palletizer_id`,`module_number`,`timestamp`,`side`),
  CONSTRAINT `palletizers_last_modules_FK` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers_sections`
--

DROP TABLE IF EXISTS `palletizers_sections`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers_sections` (
  `id` int NOT NULL AUTO_INCREMENT,
  `palletizer_id` int NOT NULL,
  `section_code` int NOT NULL,
  `name` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `palletizers_sections_unique` (`palletizer_id`,`section_code`),
  KEY `palletizers_sections_auth_group_permissions_FK` (`palletizer_id`),
  CONSTRAINT `palletizers_sections_FK` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `palletizers_status`
--

DROP TABLE IF EXISTS `palletizers_status`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `palletizers_status` (
  `id` int NOT NULL AUTO_INCREMENT,
  `palletizer_id` int NOT NULL,
  `status` tinyint NOT NULL,
  `number` tinyint NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `palletizers_status_FK` (`palletizer_id`),
  CONSTRAINT `palletizers_status_FK` FOREIGN KEY (`palletizer_id`) REFERENCES `palletizers` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `pallets`
--

DROP TABLE IF EXISTS `pallets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `pallets` (
  `id` int NOT NULL AUTO_INCREMENT,
  `order_id` int NOT NULL,
  `bobbins_amount` int NOT NULL,
  `pallet_of_order` int NOT NULL,
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `daily_id` int DEFAULT NULL,
  `product_date` date DEFAULT NULL,
  `labeling_time` timestamp NULL DEFAULT NULL,
  `team_turn` int DEFAULT NULL,
  `rfid` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `pallet_of_order` (`pallet_of_order`,`order_id`),
  KEY `order_id` (`order_id`),
  KEY `index4` (`pallet_of_order`),
  KEY `pallets_created_IDX` (`created`) USING BTREE,
  CONSTRAINT `pallets_ibfk_1` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=83291 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `pallets_tracking`
--

DROP TABLE IF EXISTS `pallets_tracking`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `pallets_tracking` (
  `id` int NOT NULL AUTO_INCREMENT,
  `section_id` int NOT NULL,
  `pallet_id` int DEFAULT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `pallets_tracking_palletizers_sections_FK` (`section_id`),
  KEY `pallets_tracking_pallets_FK` (`pallet_id`),
  CONSTRAINT `pallets_tracking_palletizers_sections_FK` FOREIGN KEY (`section_id`) REFERENCES `palletizers_sections` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `paper_tube_colors`
--

DROP TABLE IF EXISTS `paper_tube_colors`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `paper_tube_colors` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `chinese_name` text NOT NULL,
  `label_name` varchar(64) NOT NULL,
  `color1` varchar(16) DEFAULT NULL,
  `color2` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=373 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `plant_areas`
--

DROP TABLE IF EXISTS `plant_areas`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `plant_areas` (
  `code` varchar(32) NOT NULL,
  `name` varchar(32) NOT NULL,
  `position` tinyint DEFAULT NULL,
  PRIMARY KEY (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `position_types`
--

DROP TABLE IF EXISTS `position_types`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `position_types` (
  `code` varchar(64) NOT NULL,
  `name` text NOT NULL,
  `description` text NOT NULL,
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `positions`
--

DROP TABLE IF EXISTS `positions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `positions` (
  `id` int NOT NULL AUTO_INCREMENT,
  `position_type_code` varchar(64) NOT NULL,
  `code` varchar(64) NOT NULL,
  `name` text NOT NULL,
  `description` text NOT NULL,
  `places_amount` int NOT NULL,
  `position_id` int DEFAULT NULL,
  `place` int NOT NULL,
  `place_x` int NOT NULL,
  `place_y` int NOT NULL,
  `place_z` int NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`),
  KEY `position_type` (`position_type_code`),
  KEY `position_id` (`position_id`),
  CONSTRAINT `positions_FK` FOREIGN KEY (`position_type_code`) REFERENCES `position_types` (`code`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `positions_ibfk_2` FOREIGN KEY (`position_id`) REFERENCES `positions` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=12896 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `pre_defect_bobbins`
--

DROP TABLE IF EXISTS `pre_defect_bobbins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `pre_defect_bobbins` (
  `id` int NOT NULL AUTO_INCREMENT,
  `winder_id` int NOT NULL,
  `bobbin_number` int NOT NULL,
  `sorting_grade_id` int DEFAULT NULL,
  `defect_id` int DEFAULT NULL,
  `start_time` timestamp NULL DEFAULT NULL,
  `end_time` timestamp NULL DEFAULT NULL,
  `bobbin_id` bigint unsigned DEFAULT NULL,
  `doffing_id` bigint unsigned DEFAULT NULL,
  `after_sorting` tinyint(1) DEFAULT '0',
  `module_number` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `winder_id` (`winder_id`,`bobbin_number`),
  KEY `sorting_grade_id` (`sorting_grade_id`),
  KEY `defect_id` (`defect_id`),
  CONSTRAINT `pre_defect_bobbins_ibfk_5` FOREIGN KEY (`sorting_grade_id`) REFERENCES `sorting_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `pre_defect_bobbins_ibfk_6` FOREIGN KEY (`defect_id`) REFERENCES `defects` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `pre_defect_bobbins_ibfk_7` FOREIGN KEY (`winder_id`) REFERENCES `winders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=14163 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `print_servers`
--

DROP TABLE IF EXISTS `print_servers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `print_servers` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `description` text NOT NULL,
  `host` varchar(16) NOT NULL,
  `port` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=COMPACT;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `printed_pallets`
--

DROP TABLE IF EXISTS `printed_pallets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `printed_pallets` (
  `pallet_id` int NOT NULL,
  `labeling_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY `pallet_id` (`pallet_id`) USING BTREE,
  KEY `printed_pallets_labeling_time_IDX` (`labeling_time`) USING BTREE,
  CONSTRAINT `printed_pallets_ibfk_1` FOREIGN KEY (`pallet_id`) REFERENCES `pallets` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `settings`
--

DROP TABLE IF EXISTS `settings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `settings` (
  `name` varchar(64) NOT NULL,
  `value` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin,
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=COMPACT;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sorting_grades`
--

DROP TABLE IF EXISTS `sorting_grades`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sorting_grades` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` tinyint NOT NULL,
  `name` text NOT NULL,
  `chinese_name` text NOT NULL,
  `description` text NOT NULL,
  `color` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sortings`
--

DROP TABLE IF EXISTS `sortings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sortings` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `description` text NOT NULL,
  `settings` longtext NOT NULL,
  `max_modules_per_day_shift` int DEFAULT NULL,
  `max_modules_per_night_shift` int DEFAULT NULL,
  `modules_record_time` int DEFAULT NULL,
  `type` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sortings_hourly_production`
--

DROP TABLE IF EXISTS `sortings_hourly_production`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `sortings_hourly_production` (
  `sorting_id` int NOT NULL,
  `value` int DEFAULT NULL,
  `day` date NOT NULL,
  `hour` tinyint NOT NULL,
  `timestamp` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`sorting_id`,`day`,`hour`),
  CONSTRAINT `sortings_hourly_production_fk` FOREIGN KEY (`sorting_id`) REFERENCES `sortings` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `spinning_sides`
--

DROP TABLE IF EXISTS `spinning_sides`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `spinning_sides` (
  `id` int NOT NULL AUTO_INCREMENT,
  `spinning_id` int NOT NULL,
  `name` varchar(64) NOT NULL,
  `description` text NOT NULL,
  `settings` longtext NOT NULL,
  `position_code` varchar(64) NOT NULL,
  `loading_side` enum('sx','dx','sxw','dxw','fdy') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT 'sx',
  `max_modules_per_day_shift` int DEFAULT NULL,
  `max_modules_per_night_shift` int DEFAULT NULL,
  `modules_record_time` int DEFAULT NULL,
  `last_module_timestamp` timestamp NULL DEFAULT NULL,
  `max_doffings_per_day_shift` int DEFAULT NULL,
  `max_doffings_per_night_shift` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `spinning_id` (`spinning_id`),
  CONSTRAINT `spinning_sides_ibfk_1` FOREIGN KEY (`spinning_id`) REFERENCES `spinnings` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `spinnings`
--

DROP TABLE IF EXISTS `spinnings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `spinnings` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `description` text NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=7 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=COMPACT;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `team_turns`
--

DROP TABLE IF EXISTS `team_turns`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `team_turns` (
  `code` int NOT NULL,
  `name` varchar(64) NOT NULL,
  PRIMARY KEY (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `trolleys`
--

DROP TABLE IF EXISTS `trolleys`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `trolleys` (
  `id` int NOT NULL AUTO_INCREMENT,
  `number` int NOT NULL,
  `loading_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `doffing1_id` bigint NOT NULL,
  `doffing2_id` bigint NOT NULL,
  `loading_side` enum('sx','dx','sxw','dxw','fdy') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `spinning_side_id` int DEFAULT NULL,
  `reprint_time` datetime DEFAULT NULL,
  `rfid` int DEFAULT NULL,
  `sent_to_erp` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `loading_time` (`loading_time`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=15547 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vision_grades`
--

DROP TABLE IF EXISTS `vision_grades`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vision_grades` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` tinyint NOT NULL,
  `name` text NOT NULL,
  `chinese_name` text NOT NULL,
  `description` text NOT NULL,
  `color` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `warehouse_movements`
--

DROP TABLE IF EXISTS `warehouse_movements`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `warehouse_movements` (
  `module_id` int NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `movement` enum('in','out') DEFAULT NULL,
  `warehouse_id` int DEFAULT NULL,
  KEY `warehouse_movements_fk` (`warehouse_id`),
  CONSTRAINT `warehouse_movements_fk` FOREIGN KEY (`warehouse_id`) REFERENCES `warehouses` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `warehouses`
--

DROP TABLE IF EXISTS `warehouses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `warehouses` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `description` text NOT NULL,
  `settings` longtext NOT NULL,
  `position_code` varchar(64) DEFAULT NULL,
  `type` enum('fdy','dty') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `monorail_id` int NOT NULL DEFAULT '1',
  PRIMARY KEY (`id`),
  KEY `warehouses_monorails_FK` (`monorail_id`),
  CONSTRAINT `warehouses_monorails_FK` FOREIGN KEY (`monorail_id`) REFERENCES `monorails` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=COMPACT;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `warehouses_alarms`
--

DROP TABLE IF EXISTS `warehouses_alarms`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `warehouses_alarms` (
  `id` int NOT NULL AUTO_INCREMENT,
  `warehouse_id` int NOT NULL,
  `alarm_id` int NOT NULL,
  `stacker_number` tinyint NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `warehouses_alarms_FK` (`warehouse_id`),
  KEY `warehouses_alarms_FK_1` (`alarm_id`),
  CONSTRAINT `warehouses_alarms_FK` FOREIGN KEY (`warehouse_id`) REFERENCES `warehouses` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `warehouses_alarms_FK_1` FOREIGN KEY (`alarm_id`) REFERENCES `warehouses_alarms_definition` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `warehouses_alarms_definition`
--

DROP TABLE IF EXISTS `warehouses_alarms_definition`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `warehouses_alarms_definition` (
  `id` int NOT NULL AUTO_INCREMENT,
  `word` tinyint NOT NULL,
  `bit` tinyint NOT NULL,
  `name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `chinese_name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `warehouse_alarms_definition_un` (`word`,`bit`)
) ENGINE=InnoDB AUTO_INCREMENT=641 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `warehouses_cycles`
--

DROP TABLE IF EXISTS `warehouses_cycles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `warehouses_cycles` (
  `id` int NOT NULL AUTO_INCREMENT,
  `number` int DEFAULT NULL,
  `warehouse_id` int NOT NULL,
  `cycle_id` int NOT NULL,
  `cycle_number` int NOT NULL,
  `cycle_value` int NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `warehouses_cycles_UN` (`number`,`warehouse_id`,`cycle_id`,`cycle_number`),
  KEY `warehouses_cycles_FK` (`warehouse_id`),
  KEY `warehouses_cycles_FK_1` (`cycle_id`),
  CONSTRAINT `warehouses_cycles_FK` FOREIGN KEY (`warehouse_id`) REFERENCES `warehouses` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `warehouses_cycles_FK_1` FOREIGN KEY (`cycle_id`) REFERENCES `warehouses_cycles_definition` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `warehouses_cycles_definition`
--

DROP TABLE IF EXISTS `warehouses_cycles_definition`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `warehouses_cycles_definition` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(100) DEFAULT NULL,
  `chinese_name` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `warehouses_read_status`
--

DROP TABLE IF EXISTS `warehouses_read_status`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `warehouses_read_status` (
  `warehouse_id` int NOT NULL,
  `is_read` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`warehouse_id`),
  CONSTRAINT `warehouses_read_status_FK` FOREIGN KEY (`warehouse_id`) REFERENCES `warehouses` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `warehouses_status`
--

DROP TABLE IF EXISTS `warehouses_status`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `warehouses_status` (
  `id` int NOT NULL AUTO_INCREMENT,
  `warehouse_id` int NOT NULL,
  `status` tinyint NOT NULL,
  `stacker_number` tinyint NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `warehouse_status_FK` (`warehouse_id`),
  CONSTRAINT `warehouse_status_FK` FOREIGN KEY (`warehouse_id`) REFERENCES `warehouses` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `weighing_rules`
--

DROP TABLE IF EXISTS `weighing_rules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `weighing_rules` (
  `id` int NOT NULL AUTO_INCREMENT,
  `spinning_line_name` varchar(8) DEFAULT NULL,
  `winder_id` int DEFAULT NULL,
  `start_time` timestamp NULL DEFAULT NULL,
  `end_time` timestamp NULL DEFAULT NULL,
  `doffing_id` int DEFAULT NULL,
  `after_weight` tinyint(1) DEFAULT '0',
  `module_number` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `weighing_rules_ibfk_1` (`spinning_line_name`),
  KEY `winder_id` (`winder_id`),
  CONSTRAINT `weighing_rules_ibfk_1` FOREIGN KEY (`spinning_line_name`) REFERENCES `winders` (`spinning_line_name`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `weighing_rules_ibfk_2` FOREIGN KEY (`winder_id`) REFERENCES `winders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=COMPACT;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `weight_grades`
--

DROP TABLE IF EXISTS `weight_grades`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `weight_grades` (
  `id` int NOT NULL AUTO_INCREMENT,
  `code` tinyint NOT NULL,
  `name` text NOT NULL,
  `chinese_name` text NOT NULL,
  `description` text NOT NULL,
  `color` varchar(16) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `winders`
--

DROP TABLE IF EXISTS `winders`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `winders` (
  `id` int NOT NULL AUTO_INCREMENT,
  `winder_name` varchar(8) NOT NULL,
  `spinning_line_name` varchar(8) NOT NULL,
  `doff_no` int NOT NULL,
  `spinning_side_id` int NOT NULL,
  `loading_side` enum('sx','dx','sxw','dxw','fdy') CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `position` int DEFAULT '0',
  `visible` tinyint(1) NOT NULL DEFAULT '1',
  PRIMARY KEY (`id`),
  UNIQUE KEY `spinning_side_id` (`winder_name`,`spinning_line_name`,`spinning_side_id`),
  KEY `winder_name` (`winder_name`),
  KEY `machine_name` (`spinning_line_name`),
  KEY `winders_ibfk_1_idx` (`spinning_side_id`),
  CONSTRAINT `winders_ibfk_1` FOREIGN KEY (`spinning_side_id`) REFERENCES `spinning_sides` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=2729 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `winders_check`
--

DROP TABLE IF EXISTS `winders_check`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `winders_check` (
  `winders_check_time_slot_id` int NOT NULL,
  `winder_id` int NOT NULL,
  `check_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `module_number` int DEFAULT NULL,
  KEY `winders_check_FK` (`winders_check_time_slot_id`),
  KEY `winders_check_FK_1` (`winder_id`),
  CONSTRAINT `winders_check_FK` FOREIGN KEY (`winders_check_time_slot_id`) REFERENCES `winders_check_time_slot` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `winders_check_FK_1` FOREIGN KEY (`winder_id`) REFERENCES `winders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `winders_check_rules`
--

DROP TABLE IF EXISTS `winders_check_rules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `winders_check_rules` (
  `id` int NOT NULL AUTO_INCREMENT,
  `winder_id` int NOT NULL,
  `check_all` tinyint(1) NOT NULL DEFAULT '0',
  `check_next` tinyint(1) NOT NULL DEFAULT '0',
  `start_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `end_time` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `winders_check_rules_un` (`winder_id`),
  CONSTRAINT `winders_check_rules_FK` FOREIGN KEY (`winder_id`) REFERENCES `winders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `winders_check_time_slot`
--

DROP TABLE IF EXISTS `winders_check_time_slot`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `winders_check_time_slot` (
  `id` int NOT NULL AUTO_INCREMENT,
  `start_time` timestamp NOT NULL,
  `end_time` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `work_bobbins`
--

DROP TABLE IF EXISTS `work_bobbins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `work_bobbins` (
  `bobbin_id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `bobbin_id_plc` int DEFAULT NULL,
  `doffing_id` bigint unsigned NOT NULL,
  `place_in_winder` tinyint NOT NULL,
  `plant_area_code` varchar(32) DEFAULT NULL,
  `position_id` int DEFAULT NULL,
  `place` int NOT NULL,
  `sorting_grade_id` int DEFAULT NULL,
  `weight_grade_id` int DEFAULT NULL,
  `final_grade_id` int DEFAULT NULL,
  `defect_id` int DEFAULT NULL,
  `vision_grade_id` int DEFAULT NULL,
  `knitting_grade_id` int DEFAULT NULL,
  `vision_defect_id` int DEFAULT NULL,
  `to_weight` tinyint(1) NOT NULL,
  `weight` decimal(10,3) DEFAULT NULL,
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `modified` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`bobbin_id`),
  UNIQUE KEY `work_bobbins_un` (`doffing_id`,`place_in_winder`),
  KEY `doffing_id` (`doffing_id`),
  KEY `position_id` (`position_id`),
  KEY `defect_code` (`defect_id`),
  KEY `sorting_grade_id` (`sorting_grade_id`),
  KEY `work_bobbins_ibfk_6` (`weight_grade_id`),
  KEY `work_bobbins_ibfk_7` (`final_grade_id`),
  KEY `work_bobbins_place_IDX` (`place`) USING BTREE,
  KEY `work_bobbins_modified_IDX` (`modified`) USING BTREE,
  KEY `work_bobbins_created_IDX` (`created`) USING BTREE,
  KEY `work_bobbins_plant_area_code_IDX` (`plant_area_code`) USING BTREE,
  KEY `work_bobbins_bobbin_id_plc_IDX` (`bobbin_id_plc`) USING BTREE,
  KEY `work_bobbins_FK` (`knitting_grade_id`),
  KEY `work_bobbins_FK_1` (`vision_grade_id`),
  KEY `work_bobbins_FK_2` (`vision_defect_id`),
  CONSTRAINT `work_bobbins_FK` FOREIGN KEY (`knitting_grade_id`) REFERENCES `knitting_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `work_bobbins_FK_1` FOREIGN KEY (`vision_grade_id`) REFERENCES `vision_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `work_bobbins_FK_2` FOREIGN KEY (`vision_defect_id`) REFERENCES `defects` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `work_bobbins_ibfk_2` FOREIGN KEY (`doffing_id`) REFERENCES `doffings` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `work_bobbins_ibfk_3` FOREIGN KEY (`position_id`) REFERENCES `positions` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `work_bobbins_ibfk_4` FOREIGN KEY (`sorting_grade_id`) REFERENCES `sorting_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `work_bobbins_ibfk_5` FOREIGN KEY (`defect_id`) REFERENCES `defects` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `work_bobbins_ibfk_6` FOREIGN KEY (`weight_grade_id`) REFERENCES `weight_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `work_bobbins_ibfk_7` FOREIGN KEY (`final_grade_id`) REFERENCES `final_grades` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=8893623 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping routines for database 'h028'
--
/*!50003 DROP FUNCTION IF EXISTS `calc_place` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_general_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`ht_user`@`%` FUNCTION `calc_place`(`IN_old_place` INT, `IN_spindle` ENUM('A','B'), `IN_tunnel_side` VARCHAR(3), `IN_container_side` INT) RETURNS int
    NO SQL
BEGIN
	DECLARE final_place INT DEFAULT 0;

	
	IF IN_container_side = 1 THEN
		SELECT new_place
		INTO final_place FROM loading_bobbins_sequence WHERE (spindle_place = IN_old_place AND spindle = IN_spindle AND tunnel_side = IN_tunnel_side) LIMIT 1;
	END IF;

	RETURN(final_place);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP FUNCTION IF EXISTS `calc_team_turn` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`ht_user`@`%` FUNCTION `calc_team_turn`(`IN_timestamp` timestamp) RETURNS int
    NO SQL
BEGIN
	SET @dayNumber = 0;
	SET @rst = 1;
	SET @localTimestamp = IN_timestamp + INTERVAL 8 HOUR;
	SELECT dayofmonth(@localTimestamp) INTO @dayNumber;
	IF @dayNumber >= 1 AND @dayNumber <= 15 THEN
		select IF(CAST(@localTimestamp AS TIME) >= CAST('08:00:00' AS TIME) AND CAST(@localTimestamp AS TIME) < CAST('20:00:00' AS TIME), 1, 2) into @rst;

		IF @dayNumber = 1 AND CAST(@localTimestamp AS TIME) >= CAST('00:00:00' AS TIME) AND CAST(@localTimestamp AS TIME) < CAST('08:00:00' AS TIME) THEN
			SELECT 2 INTO @rst;
		END IF;

		IF @dayNumber = 15 AND CAST(@localTimestamp AS TIME) >= CAST('08:00:00' AS TIME) AND CAST(@localTimestamp AS TIME) < CAST('16:00:00' AS TIME) THEN
			SELECT 1 INTO @rst;
		END IF;
		IF @dayNumber = 15 AND CAST(@localTimestamp AS TIME) >= CAST('16:00:00' AS TIME) AND CAST(@localTimestamp AS TIME) <= CAST('23:59:59' AS TIME) THEN
			SELECT 2 INTO @rst;
		END IF;
		

	ELSEIF @dayNumber >= 16 AND @dayNumber <= 31 THEN
		select IF(CAST(@localTimestamp AS TIME) >= CAST('08:00:00' AS TIME) AND CAST(@localTimestamp AS TIME) < CAST('20:00:00' AS TIME), 2, 1) into @rst;
		
		SET @lastDayOfMonth = @dayNumber;
		SELECT dayofmonth(LAST_DAY(@localTimestamp)) INTO @lastDayOfMonth;

		IF @dayNumber = 16 AND CAST(@localTimestamp AS TIME) >= CAST('00:00:00' AS TIME) AND CAST(@localTimestamp AS TIME) < CAST('08:00:00' AS TIME) THEN
			SELECT 1 INTO @rst;
		END IF;

		IF @dayNumber = @lastDayOfMonth AND CAST(@localTimestamp AS TIME) >= CAST('08:00:00' AS TIME) AND CAST(@localTimestamp AS TIME) < CAST('16:00:00' AS TIME) THEN
			SELECT 2 INTO @rst;
		END IF;
		IF @dayNumber = @lastDayOfMonth AND CAST(@localTimestamp AS TIME) >= CAST('16:00:00' AS TIME) AND CAST(@localTimestamp AS TIME) <= CAST('23:59:59' AS TIME) THEN
			SELECT 1 INTO @rst;
		END IF;

	END IF;
	RETURN(@rst);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP FUNCTION IF EXISTS `date_from_local_date` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_general_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'NO_AUTO_VALUE_ON_ZERO' */ ;
DELIMITER ;;
CREATE DEFINER=`root`@`localhost` FUNCTION `date_from_local_date`(`IN_datetime` DATETIME) RETURNS datetime
    NO SQL
BEGIN
	DECLARE rst DATETIME DEFAULT 0;

	SELECT CONVERT_TZ(IN_datetime, (SELECT value FROM settings WHERE name = 'localTimeZone'),'+00:00') INTO rst;

	RETURN(rst);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP FUNCTION IF EXISTS `DATE_TO_LOCAL_DATE` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_general_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'NO_AUTO_VALUE_ON_ZERO' */ ;
DELIMITER ;;
CREATE DEFINER=`root`@`localhost` FUNCTION `DATE_TO_LOCAL_DATE`(`IN_datetime` DATETIME) RETURNS datetime
    NO SQL
BEGIN
	DECLARE rst DATETIME DEFAULT 0;

	
	SELECT CONVERT_TZ(IN_datetime,'+00:00', (SELECT value FROM settings WHERE name = 'localTimeZone')) INTO rst;

	RETURN(rst);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP FUNCTION IF EXISTS `date_to_shift_date` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`ht_user`@`%` FUNCTION `date_to_shift_date`(`IN_datetime` DATETIME, `IN_shift_time` VARCHAR(10)) RETURNS datetime
    NO SQL
BEGIN
	DECLARE rst DATETIME DEFAULT 0;

	
	SELECT CONVERT_TZ(IN_datetime,'+00:00', IN_shift_time) INTO rst;

	RETURN(rst);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP FUNCTION IF EXISTS `GET_BOX_CODE` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`ht_user`@`%` FUNCTION `GET_BOX_CODE`(`IN_box_id` INT) RETURNS varchar(20) CHARSET utf8mb4
    NO SQL
BEGIN
	SET @rst = '';
	SET @productDate = -1;
	SELECT DATE_FORMAT(db.product_date, '%y%m%d') INTO @productDate FROM dty_boxes AS db WHERE db.id = IN_box_id;

	SET @palletizer_code = -1;
	SELECT (SELECT ptz.code FROM palletizers AS ptz WHERE id = db.palletizer_id)
	INTO @palletizerCode FROM dty_boxes AS db WHERE db.id = IN_box_id;

  SET @palletPlantCode = -1;
  SELECT value INTO @palletPlantCode FROM settings WHERE name = 'palletPlantCode';

	SELECT CONCAT(@palletPlantCode, @productDate, @palletizerCode, LPAD(db.daily_id,4,'0') ) INTO @rst FROM dty_boxes AS db WHERE db.id = IN_box_id;
	RETURN(@rst);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP FUNCTION IF EXISTS `GET_DTY_PALLET_CODE` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`ht_user`@`%` FUNCTION `GET_DTY_PALLET_CODE`(`IN_pallet_id` INT) RETURNS varchar(20) CHARSET utf8mb4
    NO SQL
BEGIN
	SET @rst = '';
	SET @productDate = -1;
	SELECT DATE_FORMAT(p.product_date, '%y%m%d') INTO @productDate FROM dty_pallets AS p WHERE p.id = IN_pallet_id;

	SET @palletizer_code = -1;
	SELECT (SELECT ptz.code FROM palletizers AS ptz WHERE id = (SELECT palletizer_id FROM dty_orders WHERE id = p.dty_order_id))
	INTO @palletizerCode FROM dty_pallets AS p WHERE p.id = IN_pallet_id;

  SET @palletPlantCode = -1;
  SELECT value INTO @palletPlantCode FROM settings WHERE name = 'palletPlantCode';

	SELECT CONCAT(@palletPlantCode, @productDate, @palletizerCode, LPAD(p.daily_id,4,'0') ) INTO @rst FROM dty_pallets AS p WHERE p.id = IN_pallet_id;
	RETURN(@rst);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP FUNCTION IF EXISTS `GET_PALLET_CODE` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`ht_user`@`%` FUNCTION `GET_PALLET_CODE`(`IN_pallet_id` INT) RETURNS varchar(20) CHARSET utf8mb4
    NO SQL
BEGIN
	SET @rst = '';
	SET @productDate = -1;
	SELECT DATE_FORMAT(p.product_date, '%y%m%d') INTO @productDate FROM pallets AS p WHERE p.id = IN_pallet_id;

	SET @palletizer_code = -1;
	SELECT (SELECT ptz.code FROM palletizers AS ptz WHERE id = (SELECT palletizer_id FROM orders WHERE id = p.order_id))
	INTO @palletizerCode FROM pallets AS p WHERE p.id = IN_pallet_id;

  SET @palletPlantCode = -1;
  SELECT value INTO @palletPlantCode FROM settings WHERE name = 'palletPlantCode';

	SELECT CONCAT(@palletPlantCode, @productDate, @palletizerCode, LPAD(p.daily_id,4,'0') ) INTO @rst FROM pallets AS p WHERE p.id = IN_pallet_id;
	RETURN(@rst);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP FUNCTION IF EXISTS `winder_timestamp_to_datetime` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_general_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'NO_AUTO_VALUE_ON_ZERO' */ ;
DELIMITER ;;
CREATE DEFINER=`root`@`localhost` FUNCTION `winder_timestamp_to_datetime`(`IN_timestamp` INT) RETURNS datetime
    NO SQL
BEGIN
	DECLARE rst DATETIME DEFAULT 0;

	SELECT FROM_UNIXTIME(UNIX_TIMESTAMP('2000-01-01 00:00:00') + IN_timestamp) INTO rst;

	RETURN(rst);
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP PROCEDURE IF EXISTS `create_dty_warehouse_order` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`ht_user`@`%` PROCEDURE `create_dty_warehouse_order`(IN `IN_lot_id` INT, IN `IN_number_of_modules` INT, IN `IN_dty_id` INT)
BEGIN
	DECLARE EXIT HANDLER FOR SQLEXCEPTION
		BEGIN
			SELECT 'An error occured during "create_knitting_order" procedure' AS error;
			RESIGNAL;
			ROLLBACK;
		END;
	START TRANSACTION;
	SET @total_modules = 0;
	SELECT COUNT(lot_id) INTO @total_modules
	FROM modules_status AS ms
	LEFT JOIN warehouses AS w ON w.id = ms.warehouse_id
	left join lots as l on l.id = ms.lot_id
	WHERE lot_id = IN_lot_id AND timestamp < CURRENT_TIMESTAMP() - INTERVAL l.wait_time HOUR AND w.type = 'fdy' and ms.after_knitting = 0;
	
	SET @booked_for_order = 0;
	SELECT (SUM(number_of_modules) - SUM(modules_sent)) INTO @booked_for_order FROM dty_warehouse_orders
	WHERE lot_id = IN_lot_id AND status != 'completed'
	GROUP BY lot_id;
	
	IF (IN_number_of_modules > (@total_modules - @booked_for_order)) THEN
		SIGNAL SQLSTATE '45000'
		SET MESSAGE_TEXT = 'NUMBER_OF_MODULES_LOWER_THAN_AVAILABLE';
	END IF;
	
	INSERT INTO dty_warehouse_orders (lot_id, number_of_modules, dty_id) VALUES (IN_lot_id, IN_number_of_modules, IN_dty_id);
	COMMIT;
	SELECT 'true' AS message;
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP PROCEDURE IF EXISTS `create_knitting_order` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`ht_user`@`%` PROCEDURE `create_knitting_order`(IN `IN_lot_id` INT, IN `IN_number_of_modules` INT, IN `IN_knitting_id` INT)
BEGIN
	DECLARE EXIT HANDLER FOR SQLEXCEPTION
		BEGIN
			SELECT 'An error occured during "create_knitting_order" procedure' AS error;
			RESIGNAL;
			ROLLBACK;
		END;
	START TRANSACTION;
	SET @total_modules = 0;
	SELECT COUNT(lot_id) INTO @total_modules
	FROM modules_status AS ms
	LEFT JOIN warehouses AS w ON w.id = ms.warehouse_id
	left join lots as l on l.id = ms.lot_id
	WHERE lot_id = IN_lot_id AND timestamp < CURRENT_TIMESTAMP() - INTERVAL l.wait_time HOUR AND w.type = 'fdy' and ms.after_knitting = 0;
	
	SET @booked_for_order = 0;
	SELECT (SUM(number_of_modules) - SUM(modules_sent)) INTO @booked_for_order FROM knitting_orders
	WHERE lot_id = IN_lot_id AND status != 'completed'
	GROUP BY lot_id;
	
	IF (IN_number_of_modules > (@total_modules - @booked_for_order)) THEN
		SIGNAL SQLSTATE '45000'
		SET MESSAGE_TEXT = 'NUMBER_OF_MODULES_LOWER_THAN_AVAILABLE';
	END IF;
	
	INSERT INTO knitting_orders (lot_id, number_of_modules, knitting_id) VALUES (IN_lot_id, IN_number_of_modules, IN_knitting_id);
	COMMIT;
	SELECT 'true' AS message;
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP PROCEDURE IF EXISTS `load_bobbins` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`root`@`localhost` PROCEDURE `load_bobbins`(IN `IN_spinning_side_id` INT, IN `IN_id1` INT, IN `IN_id2` INT, IN `IN_number` INT, IN `IN_containerType` VARCHAR(64),in `IN_sortingId` INT, in `IN_rfid` INT)
BEGIN
	DECLARE EXIT HANDLER FOR SQLEXCEPTION
		BEGIN
			ROLLBACK;
			SELECT 'An error occured during "load_bobbins" procedure' AS error;
			RESIGNAL;
		END;

	START TRANSACTION;
		
		SET @spinning_side_id = NULL;
		IF IN_spinning_side_id IS NULL THEN
			SELECT spinning_side_id INTO @spinning_side_id FROM winders WHERE id = (SELECT winder_id FROM doffings WHERE id = IN_id1);
		ELSE
			SET @spinning_side_id = IN_spinning_side_id;
		END IF;

		SET @winderSide = '';

		SELECT loading_side INTO @winderSide FROM spinning_sides WHERE id = @spinning_side_id;

		SET @bobbin_position_id = NULL;
		SELECT id INTO @bobbin_position_id FROM positions AS p WHERE (p.position_type_code = IN_containerType AND p.place = IN_number) LIMIT 1;

		if @bobbin_position_id is not null THEN
			DELETE FROM work_bobbins WHERE position_id = @bobbin_position_id and not (doffing_id = IN_id1 or doffing_id = IN_id2);
		end if;

		UPDATE doffings SET plant_area_code = IN_containerType WHERE id = IN_id1;
		UPDATE doffings SET plant_area_code = IN_containerType WHERE id = IN_id2;
		
		UPDATE work_bobbins AS w SET 
			w.plant_area_code = IN_containerType,
			w.position_id = @bobbin_position_id,
			w.place = calc_place(w.place_in_winder, IF(w.doffing_id = IN_id1, 'A', 'B'), @winderSide, 1)
		WHERE ((w.doffing_id = IN_id1 OR w.doffing_id = IN_id2) AND NOT w.plant_area_code = IN_containerType);

		IF @bobbin_position_id IS NULL THEN
			DELETE FROM work_bobbins WHERE position_id IS null and (doffing_id = IN_id1 or doffing_id = IN_id2);
		END IF;
		
		set @last_id = -1;
		IF (IN_containerType = 'trolley') THEN
			INSERT INTO trolleys(number, doffing1_id, doffing2_id, loading_side, spinning_side_id, rfid) VALUES (IN_number, IN_id1, IN_id2, @winderSide, @spinning_side_id, IN_rfid);
			select last_insert_id() into @last_id;
		ELSEIF (IN_containerType = 'module') THEN
			
			
		
			INSERT INTO modules(number, doffing1_id, doffing2_id, loading_side, spinning_side_id, sorting_id, sorting_start_time) 
			values (IN_number, IN_id1, IN_id2, @winderSide, @spinning_side_id, IF(IN_sortingId = -1, NULL, IN_sortingId), IF(IN_sortingId = -1, NULL, CURRENT_TIMESTAMP()))
			ON DUPLICATE KEY UPDATE sorting_id = IF(sorting_id IS NOT NULL, sorting_id, IF(IN_sortingId = -1, NULL, IN_sortingId)), 
			sorting_start_time = IF(sorting_start_time IS NOT NULL, sorting_start_time, IF(IN_sortingId = -1, NULL, CURRENT_TIMESTAMP())),
			id = LAST_INSERT_ID(id);
			select last_insert_id() into @last_id;
		END IF;
	COMMIT;
	SELECT @last_id AS message;
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP PROCEDURE IF EXISTS `manage_doffing` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`root`@`localhost` PROCEDURE `manage_doffing`(IN `IN_winder_name` TEXT, IN `IN_spinning_line_name` TEXT, IN `IN_spinning_side_id` INT(11), IN `IN_doff_no` INT, IN `IN_end_time` INT, IN `IN_yarn_type` VARCHAR(12), IN `IN_order_code` TEXT, IN `IN_code_number` VARCHAR(20), IN `IN_winder_number` INT, IN `IN_warehouse_pin_number` INT)
BEGIN
	DECLARE EXIT HANDLER FOR SQLEXCEPTION
		begin
			ROLLBACK;
			SELECT 'An error occured during "manage_doffing" procedure' AS error;
			RESIGNAL;
		END;
	START TRANSACTION;
		
	
		
		IF IN_doff_no > 0 THEN
			SET @old_doff_no = -1;
			SET @winder_id = -1;
			set @winder_type = 'sx';
			SELECT loading_side into @winder_type FROM spinning_sides WHERE id = IN_spinning_side_id;
			SELECT doff_no, id INTO @old_doff_no, @winder_id FROM winders WHERE spinning_side_id = IN_spinning_side_id AND spinning_line_name = IN_spinning_line_name AND winder_name = IN_winder_name LIMIT 1;
			IF (@old_doff_no = -1) THEN
				INSERT INTO winders (winder_name, spinning_line_name, spinning_side_id, doff_no, loading_side)
				VALUES(IN_winder_name, IN_spinning_line_name, IN_spinning_side_id, IN_doff_no, @winder_type);
				SELECT LAST_INSERT_ID() INTO @winder_id;
			END IF;

			SET @lot_prefix = '';
			SELECT value INTO @lot_prefix FROM settings WHERE name = "lotPrefix";

			SET @lot_id = -1;
			SELECT id INTO @lot_id FROM lots WHERE TRIM(code) = TRIM(IN_code_number) and TRIM(order_code) = TRIM(IN_order_code) LIMIT 1;
		
			set @lot_type = if(@winder_type ='fdy', 'fdy', 'poy');
		
			SET @calcSpec = 0;
			SELECT IFNULL(TRIM(REGEXP_REPLACE(SUBSTRING_INDEX(IN_yarn_type, '/', 1),"[0-9]+", ROUND(REGEXP_SUBSTR(SUBSTRING_INDEX(IN_yarn_type, '/', 1),"[0-9]+")/1.1111, 1))),0) INTO @calcSpec;
			
			IF (@lot_id = -1) THEN
				INSERT INTO lots (prefix, code, order_code, specification_china, specification_export, name, chinese_name, standard_china, standard_export, modified, type)
				VALUES (@lot_prefix, TRIM(IN_code_number), TRIM(IN_order_code), IN_yarn_type, IN_yarn_type, '', '', '', '', null, @lot_type);
				SELECT LAST_INSERT_ID() INTO @lot_id;
			END IF;

			IF (NOT @old_doff_no = IN_doff_no) OR IN_winder_number = -1 THEN
				UPDATE winders SET doff_no = IN_doff_no, spinning_line_name = IN_spinning_line_name WHERE id = @winder_id;

				SET @plant_area_code = '';
				SET @bobbin_position_id = NULL;

				IF NOT IN_winder_number = -1 THEN
					SET @plant_area_code = 'winder';
					SELECT p1.id INTO @bobbin_position_id
					FROM positions AS p1
					WHERE p1.position_type_code = 'winder' AND p1.place = IN_winder_number AND p1.position_id = (
						SELECT id FROM positions WHERE code = (
							SELECT position_code FROM spinning_sides WHERE id = IN_spinning_side_id
						) LIMIT 1
					)
					LIMIT 1;
					
					IF @bobbin_position_id IS NOT NULL THEN
						UPDATE work_bobbins SET plant_area_code = 'winderToWarehouse', position_id = NULL
						WHERE position_id = @bobbin_position_id;
					END IF;

				ELSEIF NOT IN_warehouse_pin_number = -1 THEN
					SET @plant_area_code = 'warehousePin';
					SELECT p1.id INTO @bobbin_position_id
					FROM positions AS p1
					WHERE p1.position_type_code = 'spinningWarehousePin' AND p1.place = IN_warehouse_pin_number AND p1.position_id = (
						SELECT id FROM positions WHERE position_type_code = 'spinningWarehouse' AND position_id = (
							SELECT id FROM positions WHERE code = (
								SELECT position_code FROM spinning_sides WHERE id = IN_spinning_side_id
							) LIMIT 1
						) LIMIT 1
					)
					LIMIT 1;

					

					IF @bobbin_position_id IS NOT NULL THEN
						UPDATE work_bobbins SET plant_area_code = 'warehouseToLoad', position_id = NULL
						WHERE position_id = @bobbin_position_id;
					END IF;
				END IF;

				SET @doffing_end_time = current_timestamp() ;
				SET @currentDailyWinderDoffNo = 0;
				
				SELECT COUNT(*) INTO @currentDailyWinderDoffNo FROM doffings WHERE winder_id = @winder_id 
				AND created <= current_timestamp() AND created >= current_timestamp - INTERVAL 12 HOUR
				AND team_turn = CALC_TEAM_TURN(current_timestamp());
				SET @doffing_id = -1;
                INSERT INTO doffings (winder_id, doff_no, end_time, yarn_type, code_number, lot_id, plant_area_code, shift_doff_no)
				VALUES (@winder_id, IN_doff_no, @doffing_end_time, IN_yarn_type, IN_code_number, @lot_id, @plant_area_code, @currentDailyWinderDoffNo + 1)
				on DUPLICATE key update id = LAST_INSERT_ID(id);

				
				SELECT LAST_INSERT_ID() INTO @doffing_id;

				SET @i = 1;
				WHILE @i <= 12 DO
					SET @sorting_grade_id = 1;
					SET @defect_id = NULL;
					SET @pre_defect_bobbin_id = -1;

					SELECT id, sorting_grade_id, defect_id INTO @pre_defect_bobbin_id, @sorting_grade_id, @defect_id FROM pre_defect_bobbins WHERE winder_id = @winder_id
					AND bobbin_number = @i
					AND (winder_id IS NULL OR winder_id = @winder_id)
					AND (start_time IS NULL OR @doffing_end_time >= start_time) AND
					(end_time IS NULL OR @doffing_end_time <= end_time)
					AND bobbin_id IS NULL
					AND doffing_id IS NULL;

					INSERT INTO work_bobbins (doffing_id, place_in_winder, plant_area_code, position_id, place, sorting_grade_id, defect_id, to_weight, weight)
					VALUES (@doffing_id, @i, @plant_area_code, @bobbin_position_id, @i, @sorting_grade_id, @defect_id, 0, NULL)
					on DUPLICATE key update bobbin_id = bobbin_id;

					SET @bobbin_id = LAST_INSERT_ID();
					UPDATE work_bobbins SET bobbin_id_plc = (bobbin_id & b'00111111111111111111111111111111') + 1, modified = NULL WHERE bobbin_id = @bobbin_id;

					IF NOT @pre_defect_bobbin_id = -1 THEN
						SET @pre_defect_end_time = NULL;
						SELECT pdb.end_time INTO @pre_defect_end_time FROM pre_defect_bobbins pdb WHERE pdb.id = @pre_defect_bobbin_id;
						SELECT IF(@pre_defect_end_time IS NULL, current_timestamp(), @pre_defect_end_time) INTO @pre_defect_end_time;
						UPDATE pre_defect_bobbins SET bobbin_id = @bobbin_id, doffing_id = @doffing_id, end_time = @pre_defect_end_time WHERE id = @pre_defect_bobbin_id;
					END IF;

                    IF @bobbin_position_id IS NULL THEN
						DELETE FROM work_bobbins WHERE bobbin_id = @bobbin_id;
                    END IF;
                   
					SET @i = @i + 1;
				END WHILE;
			END IF;
		END IF;
	COMMIT;
	SELECT 'true' AS message;
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP PROCEDURE IF EXISTS `manage_dty_bobbins` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`root`@`localhost` PROCEDURE `manage_dty_bobbins`(in `IN_module_number` INT)
BEGIN
	DECLARE EXIT HANDLER FOR SQLEXCEPTION
		begin
			ROLLBACK;
			SELECT 'An error occured during "manage_dty_bobbins" procedure' AS error;
			RESIGNAL;
		END;
	START TRANSACTION;
		set @moduleId = -1;
		insert into dty_modules (module_number) values (IN_module_number);
		select last_insert_id() into @moduleId;
		
		SET @i = 1;
		while @i <= 96 DO
			set @bobbinId = -1;
			insert into dty_bobbins values ();
			select last_insert_id() into @bobbinId; 
			insert into dty_module_bobbins values (@moduleId, @bobbinId);
			set @i = @i + 1;
		end while;
	COMMIT;
	SELECT 'true' AS message;
END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 DROP PROCEDURE IF EXISTS `manage_negative_doffing` */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_general_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
CREATE DEFINER=`root`@`localhost` PROCEDURE `manage_negative_doffing`(IN `IN_code_number` VARCHAR(12), IN `IN_module_number` INT)
BEGIN
	DECLARE EXIT HANDLER FOR SQLEXCEPTION
		BEGIN
			SELECT 'An error occured during "manage_negative_doffing" procedure' AS error;
			RESIGNAL;
			ROLLBACK;

		END;

	START TRANSACTION;
		SET @lot_prefix = '';
		SELECT value INTO @lot_prefix FROM settings WHERE name = "lotPrefix";

		SET @lot_id = -1;
		SELECT id INTO @lot_id FROM lots WHERE  TRIM(code) = TRIM(IN_code_number) LIMIT 1;
		IF NOT @lot_id = -1 THEN


			SET @plant_area_code = '';
			SET @bobbin_position_id = NULL;

			IF NOT IN_module_number = -1 THEN
				SET @plant_area_code = 'module';
				SELECT p1.id INTO @bobbin_position_id
				FROM positions AS p1
				WHERE p1.position_type_code = 'module' AND p1.place = IN_module_number;
			END IF;
			IF @bobbin_position_id IS NOT NULL THEN
				DELETE FROM work_bobbins WHERE position_id = @bobbin_position_id;
			END IF;

			INSERT INTO doffings (code_number, lot_id, plant_area_code)
			VALUES (IN_code_number, @lot_id, @plant_area_code);

			SET @doffing1_id = -1;
			SELECT LAST_INSERT_ID() INTO @doffing1_id;

			INSERT INTO doffings (code_number, lot_id, plant_area_code)
			VALUES (IN_code_number, @lot_id, @plant_area_code);

			SET @doffing2_id = -1;
			SELECT LAST_INSERT_ID() INTO @doffing2_id;

			INSERT INTO modules (number, doffing1_id, doffing2_id) VALUES (IN_module_number, @doffing1_id, @doffing2_id);

			SET @i = 1;
			WHILE @i <= 12 DO

				INSERT INTO work_bobbins (doffing_id, place_in_winder, plant_area_code, position_id, place, sorting_grade_id, weight_grade_id, final_grade_id, defect_id, to_weight, weight)
				VALUES (@doffing1_id, @i, @plant_area_code, @bobbin_position_id, @i, NULL, NULL, NULL, NULL, 0, NULL);

				SET @bobbin_id = LAST_INSERT_ID();
				UPDATE work_bobbins SET bobbin_id_plc = (bobbin_id & b'00111111111111111111111111111111') + 1, modified = NULL WHERE bobbin_id = @bobbin_id;

				IF @bobbin_position_id IS NULL THEN
					DELETE FROM work_bobbins WHERE bobbin_id = @bobbin_id;
				END IF;

				SET @i = @i + 1;
			END WHILE;

			SET @i = 1;
			WHILE @i <= 12 DO

				INSERT INTO work_bobbins (doffing_id, place_in_winder, plant_area_code, position_id, place, sorting_grade_id, weight_grade_id, final_grade_id, defect_id, to_weight, weight)
				VALUES (@doffing2_id, @i, @plant_area_code, @bobbin_position_id, @i+12, NULL, NULL, NULL, NULL, 0, NULL);

				SET @bobbin_id = LAST_INSERT_ID();
				UPDATE work_bobbins SET bobbin_id_plc = (bobbin_id & b'00111111111111111111111111111111') + 1, modified = NULL WHERE bobbin_id = @bobbin_id;

				IF @bobbin_position_id IS NULL THEN
					DELETE FROM work_bobbins WHERE bobbin_id = @bobbin_id;
				END IF;

				SET @i = @i + 1;
			END WHILE;
		END IF;
	COMMIT;

	SELECT 'true' AS message;

END ;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;

--
-- Final view structure for view `current_modules`
--

/*!50001 DROP VIEW IF EXISTS `current_modules`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`root`@`localhost` SQL SECURITY DEFINER */
/*!50001 VIEW `current_modules` AS select `p`.`place` AS `number`,`l`.`id` AS `lot_id`,count(`w`.`bobbin_id`) AS `bobbins` from (((`positions` `p` left join `work_bobbins` `w` on((`w`.`position_id` = `p`.`id`))) left join `doffings` `d` on((`w`.`doffing_id` = `d`.`id`))) left join `lots` `l` on((trim(`d`.`code_number`) = `l`.`code`))) where (`p`.`position_type_code` = 'module') group by `p`.`code`,`w`.`position_id` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `current_trolleys`
--

/*!50001 DROP VIEW IF EXISTS `current_trolleys`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`root`@`localhost` SQL SECURITY DEFINER */
/*!50001 VIEW `current_trolleys` AS select `p`.`place` AS `number`,`l`.`id` AS `lot_id`,count(`w`.`bobbin_id`) AS `bobbins` from (((`positions` `p` left join `work_bobbins` `w` on((`w`.`position_id` = `p`.`id`))) left join `doffings` `d` on((`w`.`doffing_id` = `d`.`id`))) left join `lots` `l` on((`d`.`code_number` = `l`.`code`))) where (`p`.`position_type_code` = 'trolley') group by `p`.`code`,`w`.`position_id` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `customer_spinning_lines`
--

/*!50001 DROP VIEW IF EXISTS `customer_spinning_lines`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_0900_ai_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`ht_user`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `customer_spinning_lines` AS select distinct `t`.`customer_line_name` AS `customer_line_name` from (select `winders`.`spinning_line_name` AS `spinning_line_name`,substr(`winders`.`spinning_line_name`,1,locate('L',`winders`.`spinning_line_name`)) AS `customer_line_name` from `winders`) `t` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `erp_interface`
--

/*!50001 DROP VIEW IF EXISTS `erp_interface`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8 */;
/*!50001 SET character_set_results     = utf8 */;
/*!50001 SET collation_connection      = utf8_general_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`root`@`localhost` SQL SECURITY DEFINER */
/*!50001 VIEW `erp_interface` AS select `p`.`id` AS `id`,(select `settings`.`value` from `settings` where (`settings`.`name` = 'companyCode')) AS `GSH`,'自动包装线' AS `SJLY`,`GET_PALLET_CODE`(`p`.`id`) AS `TM`,concat(`l`.`prefix`,'-',`l`.`code`) AS `BZPH`,`l`.`specification_china` AS `GG`,`og`.`chinese_name` AS `DJ`,`lw`.`net` AS `JZ`,`lw`.`gross` AS `MZ`,`p`.`bobbins_amount` AS `TS`,`ptc`.`chinese_name` AS `GS`,(select (case `p`.`team_turn` when '1' then '甲' when '2' then '乙' when '3' then '丙' else '' end)) AS `BB`,if(((cast(`DATE_TO_LOCAL_DATE`(`p`.`created`) as time) >= cast('8:00:00' as time)) and (cast(`DATE_TO_LOCAL_DATE`(`p`.`created`) as time) < cast('20:00:00' as time))),'日','夜') AS `BC`,`p`.`product_date` AS `RQ`,lpad(`o`.`operator_number`,3,'0') AS `GH`,`DATE_TO_LOCAL_DATE`(`p`.`created`) AS `XTSJ`,'N' AS `ERPYY`,`ptz`.`code` AS `XB`,if((`o`.`destination` = 1),'NDPOY','WDPOY') AS `NWX`,`p`.`id` AS `IDP`,'Y' AS `SFJLK` from ((((((`pallets` `p` left join `orders` `o` on((`p`.`order_id` = `o`.`id`))) left join `lots` `l` on((`o`.`lot_id` = `l`.`id`))) left join `lot_weights` `lw` on(((`lw`.`lot_id` = `l`.`id`) and (`lw`.`level` = `o`.`pallet_level`)))) left join `order_grades` `og` on((`o`.`order_grade_id` = `og`.`id`))) left join `paper_tube_colors` `ptc` on((`l`.`paper_tube_color_id` = `ptc`.`id`))) left join `palletizers` `ptz` on((`o`.`palletizer_id` = `ptz`.`id`))) */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `spinning_lines`
--

/*!50001 DROP VIEW IF EXISTS `spinning_lines`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_0900_ai_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`ht_user`@`%` SQL SECURITY DEFINER */
/*!50001 VIEW `spinning_lines` AS select distinct `winders`.`spinning_line_name` AS `name`,`winders`.`spinning_side_id` AS `spinning_side_id` from `winders` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2025-07-15  4:19:06
