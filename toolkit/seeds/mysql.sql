-- Standard e-commerce seed for testing the MySQL dialect in the app.
-- Exercises: INT/VARCHAR/TEXT/DECIMAL/DATETIME/ENUM, PK, UNIQUE, FK (RESTRICT
-- and CASCADE), multi-column unique, and indexes. Run against an empty db:
--   mysql -uroot orders < dev/seeds/mysql.sql

DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS customers;

CREATE TABLE customers (
  id INT PRIMARY KEY AUTO_INCREMENT,
  email VARCHAR(255) NOT NULL UNIQUE,
  full_name VARCHAR(120) NOT NULL,
  status ENUM('active','suspended','deleted') NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE products (
  id INT PRIMARY KEY AUTO_INCREMENT,
  sku VARCHAR(32) NOT NULL UNIQUE,
  name VARCHAR(200) NOT NULL,
  category ENUM('apparel','electronics','home','books','other') NOT NULL DEFAULT 'other',
  price DECIMAL(10,2) NOT NULL,
  stock INT NOT NULL DEFAULT 0,
  description TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orders (
  id INT PRIMARY KEY AUTO_INCREMENT,
  customer_id INT NOT NULL,
  status ENUM('pending','paid','shipped','delivered','cancelled','refunded') NOT NULL DEFAULT 'pending',
  total DECIMAL(12,2) NOT NULL DEFAULT 0.00,
  placed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE RESTRICT
);
CREATE INDEX idx_orders_customer ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);

CREATE TABLE order_items (
  id INT PRIMARY KEY AUTO_INCREMENT,
  order_id INT NOT NULL,
  product_id INT NOT NULL,
  quantity INT NOT NULL,
  unit_price DECIMAL(10,2) NOT NULL,
  FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
  FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT,
  UNIQUE KEY uniq_order_product (order_id, product_id)
);

INSERT INTO customers (email, full_name, status) VALUES
  ('alice@example.com', 'Alice Martin',   'active'),
  ('bob@example.com',   'Bob Tanaka',     'active'),
  ('carol@example.com', 'Carol Singh',    'suspended'),
  ('dave@example.com',  'Dave Okafor',    'active'),
  ('eve@example.com',   'Eve Larsson',    'deleted');

INSERT INTO products (sku, name, category, price, stock, description) VALUES
  ('SHIRT-001', 'Cotton T-Shirt',      'apparel',     19.99, 120, 'Soft cotton tee, unisex.'),
  ('JEAN-014',  'Slim Fit Jeans',      'apparel',     59.00,  40, NULL),
  ('HDPH-200',  'Wireless Headphones', 'electronics',129.50,  25, 'Bluetooth 5.3, 30h battery.'),
  ('LMP-009',   'Desk Lamp',           'home',        34.95,  80, NULL),
  ('BOOK-SQL',  'Learning SQL',        'books',       24.00, 200, '3rd edition.'),
  ('MOUSE-77',  'Ergonomic Mouse',     'electronics', 45.00,   0, 'Currently out of stock.');

INSERT INTO orders (customer_id, status, total, placed_at) VALUES
  (1, 'delivered', 78.99,  '2026-04-12 10:14:00'),
  (1, 'shipped',  129.50,  '2026-05-01 09:02:00'),
  (2, 'paid',      24.00,  '2026-05-10 18:45:00'),
  (2, 'pending',    0.00,  '2026-05-26 08:00:00'),
  (4, 'cancelled', 45.00,  '2026-04-30 14:22:00'),
  (4, 'refunded',  19.99,  '2026-05-05 11:11:00');

INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES
  (1, 1, 1, 19.99),
  (1, 2, 1, 59.00),
  (2, 3, 1, 129.50),
  (3, 5, 1, 24.00),
  (5, 6, 1, 45.00),
  (6, 1, 1, 19.99);

DROP FUNCTION IF EXISTS order_total;
DROP PROCEDURE IF EXISTS cancel_order;

DELIMITER //

CREATE FUNCTION order_total(oid INT) RETURNS DECIMAL(12,2) DETERMINISTIC
BEGIN
  DECLARE t DECIMAL(12,2);
  SELECT COALESCE(SUM(quantity * unit_price), 0) INTO t
  FROM order_items WHERE order_id = oid;
  RETURN t;
END//

CREATE PROCEDURE cancel_order(IN oid INT)
BEGIN
  UPDATE orders SET status = 'cancelled' WHERE id = oid;
END//

DELIMITER ;
