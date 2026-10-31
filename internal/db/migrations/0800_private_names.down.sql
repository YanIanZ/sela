-- Reverts 0800_private_names. The list is owner-authored data with no other
-- table depending on it; deleting the owner crypto-shreds old copies.
DROP TABLE IF EXISTS private_names;
