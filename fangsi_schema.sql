-- Table structure for table `doffings`
--

DROP TABLE IF EXISTS `doffings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `doffings` (
  `id` int(11) NOT NULL COMMENT '流水号',
  `lot_code` varchar(64) DEFAULT NULL COMMENT '批号',
  `order_code` varchar(64) DEFAULT NULL COMMENT '工单编号',
  `specification` varchar(64) DEFAULT NULL COMMENT '规格',
  `line_name` varchar(64) DEFAULT NULL COMMENT '落筒线',
  `winder_name` varchar(64) DEFAULT NULL COMMENT '卷绕头名称',
  `created` timestamp NULL DEFAULT NULL COMMENT '创建时间',
  `paper_tube` varchar(64) DEFAULT NULL COMMENT '纸管',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `modules`
--

DROP TABLE IF EXISTS `modules`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `modules` (
  `id` int(11) NOT NULL COMMENT '流水号',
  `module_number` int(11) DEFAULT NULL COMMENT '吊车号',
  `loading_time` timestamp NULL DEFAULT NULL COMMENT '装载时间',
  `lot_code` varchar(64) DEFAULT NULL COMMENT '批号',
  `order_code` varchar(64) DEFAULT NULL COMMENT '工单编号',
  `doffing_1_id` int(11) DEFAULT NULL,
  `doffing_2_id` int(11) DEFAULT NULL,
  `line_name_1` varchar(64) DEFAULT NULL COMMENT '落筒线',
  `winder_name_1` varchar(64) DEFAULT NULL COMMENT '卷绕头名称',
  `line_name_2` varchar(64) DEFAULT NULL COMMENT '落筒线',
  `winder_name_2` varchar(64) DEFAULT NULL COMMENT '卷绕头名称',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `trolleys`
--

DROP TABLE IF EXISTS `trolleys`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `trolleys` (
  `id` int(11) NOT NULL COMMENT '流水号',
  `trolley_number` int(11) DEFAULT NULL COMMENT '丝车号',
  `loading_time` timestamp NULL DEFAULT NULL COMMENT '装载时间',
  `lot_code` varchar(64) DEFAULT NULL COMMENT '批号',
  `order_code` varchar(64) DEFAULT NULL COMMENT '工单编号',
  `doffing_1_id` int(11) DEFAULT NULL,
  `doffing_2_id` int(11) DEFAULT NULL,
  `line_name_1` varchar(64) DEFAULT NULL COMMENT '落筒线',
  `winder_name_1` varchar(64) DEFAULT NULL COMMENT '卷绕头名称',
  `line_name_2` varchar(64) DEFAULT NULL COMMENT '落筒线',
  `winder_name_2` varchar(64) DEFAULT NULL COMMENT '卷绕头名称',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
