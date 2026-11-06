INSERT INTO products (sku, name, price, stock, is_active) VALUES
    ('ELC-001', 'Laptop',      8500000,  25, TRUE),
    ('ELC-002', 'Smartphone',  4200000,  60, TRUE),
    ('ELC-003', 'Monitor',     2150000,  40, TRUE),
    ('ELC-004', 'Keyboard',     450000, 120, TRUE),
    ('ELC-005', 'Mouse',        250000, 200, TRUE),
    ('ELC-006', 'Headphone',    750000,  80, TRUE),
    ('ELC-007', 'Tablet',      3600000,  35, TRUE),
    -- low stock (<= 10)
    ('ELC-008', 'Printer',     1850000,   8, TRUE),
    ('ELC-009', 'Speaker',      950000,   5, TRUE),
    ('ELC-010', 'Webcam',       600000,  10, TRUE),
    -- inactive
    ('ELC-011', 'Router',       550000,  15, FALSE),
    -- no movement
    ('ELC-012', 'Smartwatch',  2300000,   0, TRUE);

-- insert initial movements
INSERT INTO stock_movements (product_id, type, qty, note, created_at)
SELECT id, 'in', stock, 'initial stock', now() - interval '30 days'
FROM products
WHERE stock > 0;

-- insert #1 order
INSERT INTO sales_orders (customer, total, created_at) VALUES 
    ('Budi Santoso', 9000000, now() - interval '2 days');

INSERT INTO sales_order_items (order_id, sku, qty, unit_price) VALUES
    ((SELECT id from sales_orders WHERE customer = 'Budi Santoso'), 'ELC-001', 1, (SELECT price FROM products WHERE sku = 'ELC-001')),
    ((SELECT id from sales_orders WHERE customer = 'Budi Santoso'), 'ELC-005', 2, (SELECT price FROM products WHERE sku = 'ELC-005'));

INSERT INTO stock_movements (product_id, type, qty, note, created_at) VALUES
    ((SELECT id FROM products WHERE sku = 'ELC-001'), 'out', 1, 'order #1 Budi', now() - interval '2 days'),
    ((SELECT id FROM products WHERE sku = 'ELC-005'), 'out', 2, 'order #1 Budi', now() - interval '2 days');

UPDATE products p SET stock = p.stock - i.qty 
FROM sales_order_items i 
JOIN sales_orders o ON o.id = i.order_id
WHERE i.sku = p.sku AND o.customer = 'Budi Santoso';

-- insert #2 order
INSERT INTO sales_orders (customer, total, created_at) VALUES 
    ('Siti Rahma', 4300000, now() - interval '3 days');

INSERT INTO sales_order_items (order_id, sku, qty, unit_price) VALUES
    ((SELECT id from sales_orders WHERE customer = 'Siti Rahma'), 'ELC-003', 2, (SELECT price FROM products WHERE sku = 'ELC-003'));

INSERT INTO stock_movements (product_id, type, qty, note, created_at) VALUES
    ((SELECT id FROM products WHERE sku = 'ELC-003'), 'out', 2, 'order #2 Siti', now() - interval '3 days');

UPDATE products p SET stock = p.stock - i.qty 
FROM sales_order_items i 
JOIN sales_orders o ON o.id = i.order_id
WHERE i.sku = p.sku AND o.customer = 'Siti Rahma';