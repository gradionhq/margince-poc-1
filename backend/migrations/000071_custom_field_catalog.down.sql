-- 000071 down — drop the tenant-isolation policy and the custom_field table cleanly.
DROP POLICY IF EXISTS custom_field_tenant_isolation ON custom_field;
DROP TABLE IF EXISTS custom_field;
