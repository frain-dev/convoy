-- +migrate Up

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION validate_instance_overrides_scope()
    RETURNS TRIGGER AS $$
BEGIN
    IF NEW.scope_type = 'project' THEN
        IF NOT EXISTS (
            SELECT 1 FROM convoy.projects WHERE id = NEW.scope_id
        ) THEN
            RAISE EXCEPTION 'Invalid scope_id: % for scope_type: project', NEW.scope_id;
        END IF;
    ELSIF NEW.scope_type = 'organisation' THEN
        IF NOT EXISTS (
            SELECT 1 FROM convoy.organisations WHERE id = NEW.scope_id
        ) THEN
            RAISE EXCEPTION 'Invalid scope_id: % for scope_type: organisation', NEW.scope_id;
        END IF;
    ELSE
        RAISE EXCEPTION 'Invalid scope_type: %', NEW.scope_type;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION handle_project_delete()
    RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM convoy.instance_overrides WHERE scope_id = OLD.id AND scope_type = 'project';
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION handle_organisation_delete()
    RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM convoy.instance_overrides WHERE scope_id = OLD.id AND scope_type = 'organisation';
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd

CREATE TRIGGER validate_scope_trigger
    BEFORE INSERT OR UPDATE ON convoy.instance_overrides
    FOR EACH ROW
EXECUTE FUNCTION validate_instance_overrides_scope();

CREATE TRIGGER trigger_delete_project
    AFTER DELETE ON convoy.projects
    FOR EACH ROW EXECUTE FUNCTION handle_project_delete();

CREATE TRIGGER trigger_delete_organisation
    AFTER DELETE ON convoy.organisations
    FOR EACH ROW EXECUTE FUNCTION handle_organisation_delete();

-- +migrate Down
DROP TRIGGER IF EXISTS validate_scope_trigger ON convoy.instance_overrides;
DROP TRIGGER IF EXISTS trigger_delete_project ON convoy.projects;
DROP TRIGGER IF EXISTS trigger_delete_organisation ON convoy.organisations;
DROP FUNCTION IF EXISTS validate_instance_overrides_scope;
DROP FUNCTION IF EXISTS handle_project_delete;
DROP FUNCTION IF EXISTS handle_organisation_delete;
