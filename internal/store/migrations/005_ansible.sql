-- Add Ansible playbook file path to playbooks table.
-- When file_path is set the row represents an Ansible playbook on disk;
-- the playbook_steps rows are ignored for that playbook.
ALTER TABLE playbooks ADD COLUMN file_path TEXT NOT NULL DEFAULT '';
