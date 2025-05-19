ALTER TABLE `campaigns` ADD COLUMN smtp_id bigint;
DROP TABLE `smtp`;
CREATE TABLE `smtp`(
	id integer primary key auto_increment,
	user_id bigint,
	interface_type varchar(255),
	name varchar(255),
	host varchar(255),
	username varchar(255),
	password varchar(255),
	from_address varchar(255),
	modified_date datetime,
	ignore_cert_errors BOOLEAN
);
