-- Reduce the order_status Classifier from 8 codes to 5 (Inquiry / In Queue / In Progress /
-- Completed / Archived). Remap existing Order rows first, then drop the retired Classifier
-- codes - internal/seed/classifiers.go's additive-only seeding (INSERT OR IGNORE) never removes
-- codes on its own, so the cleanup has to happen here. Scoped to type = 'order_status' only.
UPDATE "Order" SET status = 'in_queue' WHERE status = 'waiting_dropoff';
UPDATE "Order" SET status = 'in_progress' WHERE status = 'waiting_on_client';
UPDATE "Order" SET status = 'completed' WHERE status IN ('waiting_payment', 'ready_for_pickup');

DELETE FROM Classifier WHERE type = 'order_status'
  AND code IN ('waiting_dropoff', 'waiting_on_client', 'waiting_payment', 'ready_for_pickup');
