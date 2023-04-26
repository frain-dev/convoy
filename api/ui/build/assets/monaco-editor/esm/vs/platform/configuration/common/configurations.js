import { addToValueTree, toValuesTree } from './configuration.js';
import { ConfigurationModel } from './configurationModels.js';
import { Extensions, overrideIdentifiersFromKey, OVERRIDE_PROPERTY_REGEX } from './configurationRegistry.js';
import { Registry } from '../../registry/common/platform.js';
export class DefaultConfigurationModel extends ConfigurationModel {
    constructor(configurationDefaultsOverrides = {}) {
        const properties = Registry.as(Extensions.Configuration).getConfigurationProperties();
        const keys = Object.keys(properties);
        const contents = Object.create(null);
        const overrides = [];
        for (const key in properties) {
            const defaultOverrideValue = configurationDefaultsOverrides[key];
            const value = defaultOverrideValue !== undefined ? defaultOverrideValue : properties[key].default;
            addToValueTree(contents, key, value, message => console.error(`Conflict in default settings: ${message}`));
        }
        for (const key of Object.keys(contents)) {
            if (OVERRIDE_PROPERTY_REGEX.test(key)) {
                overrides.push({
                    identifiers: overrideIdentifiersFromKey(key),
                    keys: Object.keys(contents[key]),
                    contents: toValuesTree(contents[key], message => console.error(`Conflict in default settings file: ${message}`)),
                });
            }
        }
        super(contents, keys, overrides);
    }
}
