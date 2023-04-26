/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { editorConfigurationBaseNode } from '../../../common/config/editorConfigurationSchema.js';
import * as nls from '../../../../nls.js';
import { Extensions } from '../../../../platform/configuration/common/configurationRegistry.js';
import { Registry } from '../../../../platform/registry/common/platform.js';
Registry.as(Extensions.Configuration).registerConfiguration(Object.assign(Object.assign({}, editorConfigurationBaseNode), { properties: {
        'editor.experimental.useCustomCodeActionMenu': {
            type: 'boolean',
            tags: ['experimental'],
            scope: 5 /* ConfigurationScope.LANGUAGE_OVERRIDABLE */,
            description: nls.localize('codeActionWidget', "Enabling this adjusts how the code action menu is rendered."),
            default: false,
        },
    } }));
