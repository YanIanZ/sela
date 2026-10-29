-- Reverts 0900_ops_canary: removes the canary and its data key row.
DELETE FROM user_data_keys WHERE user_id IN (SELECT key_id FROM ops_canary);
DROP TABLE IF EXISTS ops_canary;
