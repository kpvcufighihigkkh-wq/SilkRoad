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
