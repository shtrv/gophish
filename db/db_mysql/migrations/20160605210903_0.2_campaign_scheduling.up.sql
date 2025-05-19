ALTER TABLE `campaigns` ADD COLUMN launch_date DATETIME;

UPDATE `campaigns` SET launch_date = created_date;
