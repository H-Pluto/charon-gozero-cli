-- 000001_init —— 建 charon 的基础表。
--
-- 这个迁移当前只建了脚手架自证用的 demo_item。
-- 业务开工后：
--   1. 删除 demo_item（写一个新的迁移来删，不要改这个文件——
--      已应用的迁移不允许修改，否则各环境的 schema 会不一致）
--   2. 追加你自己的建表语句

-- demo_item 是「整条链路打通」的验证表：config → svc → repo → model → 接口。
CREATE TABLE IF NOT EXISTS demo_item (
    id         BIGSERIAL    PRIMARY KEY,
    name       VARCHAR(64)  NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- 按创建时间倒序分页是列表接口的默认行为，建索引避免全表排序。
CREATE INDEX IF NOT EXISTS idx_demo_item_created_at ON demo_item (created_at DESC);

COMMENT ON TABLE demo_item IS '脚手架自证用表，业务开工后删除';
