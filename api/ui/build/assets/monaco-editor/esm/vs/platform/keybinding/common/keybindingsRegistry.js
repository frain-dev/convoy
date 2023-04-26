/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { createKeybinding } from '../../../base/common/keybindings.js';
import { OS } from '../../../base/common/platform.js';
import { CommandsRegistry } from '../../commands/common/commands.js';
import { Registry } from '../../registry/common/platform.js';
class KeybindingsRegistryImpl {
    constructor() {
        this._coreKeybindings = [];
        this._extensionKeybindings = [];
        this._cachedMergedKeybindings = null;
    }
    /**
     * Take current platform into account and reduce to primary & secondary.
     */
    static bindToCurrentPlatform(kb) {
        if (OS === 1 /* OperatingSystem.Windows */) {
            if (kb && kb.win) {
                return kb.win;
            }
        }
        else if (OS === 2 /* OperatingSystem.Macintosh */) {
            if (kb && kb.mac) {
                return kb.mac;
            }
        }
        else {
            if (kb && kb.linux) {
                return kb.linux;
            }
        }
        return kb;
    }
    registerKeybindingRule(rule) {
        const actualKb = KeybindingsRegistryImpl.bindToCurrentPlatform(rule);
        if (actualKb && actualKb.primary) {
            const kk = createKeybinding(actualKb.primary, OS);
            if (kk) {
                this._registerDefaultKeybinding(kk, rule.id, rule.args, rule.weight, 0, rule.when);
            }
        }
        if (actualKb && Array.isArray(actualKb.secondary)) {
            for (let i = 0, len = actualKb.secondary.length; i < len; i++) {
                const k = actualKb.secondary[i];
                const kk = createKeybinding(k, OS);
                if (kk) {
                    this._registerDefaultKeybinding(kk, rule.id, rule.args, rule.weight, -i - 1, rule.when);
                }
            }
        }
    }
    registerCommandAndKeybindingRule(desc) {
        this.registerKeybindingRule(desc);
        CommandsRegistry.registerCommand(desc);
    }
    static _mightProduceChar(keyCode) {
        if (keyCode >= 21 /* KeyCode.Digit0 */ && keyCode <= 30 /* KeyCode.Digit9 */) {
            return true;
        }
        if (keyCode >= 31 /* KeyCode.KeyA */ && keyCode <= 56 /* KeyCode.KeyZ */) {
            return true;
        }
        return (keyCode === 80 /* KeyCode.Semicolon */
            || keyCode === 81 /* KeyCode.Equal */
            || keyCode === 82 /* KeyCode.Comma */
            || keyCode === 83 /* KeyCode.Minus */
            || keyCode === 84 /* KeyCode.Period */
            || keyCode === 85 /* KeyCode.Slash */
            || keyCode === 86 /* KeyCode.Backquote */
            || keyCode === 110 /* KeyCode.ABNT_C1 */
            || keyCode === 111 /* KeyCode.ABNT_C2 */
            || keyCode === 87 /* KeyCode.BracketLeft */
            || keyCode === 88 /* KeyCode.Backslash */
            || keyCode === 89 /* KeyCode.BracketRight */
            || keyCode === 90 /* KeyCode.Quote */
            || keyCode === 91 /* KeyCode.OEM_8 */
            || keyCode === 92 /* KeyCode.IntlBackslash */);
    }
    _assertNoCtrlAlt(keybinding, commandId) {
        if (keybinding.ctrlKey && keybinding.altKey && !keybinding.metaKey) {
            if (KeybindingsRegistryImpl._mightProduceChar(keybinding.keyCode)) {
                console.warn('Ctrl+Alt+ keybindings should not be used by default under Windows. Offender: ', keybinding, ' for ', commandId);
            }
        }
    }
    _registerDefaultKeybinding(keybinding, commandId, commandArgs, weight1, weight2, when) {
        if (OS === 1 /* OperatingSystem.Windows */) {
            this._assertNoCtrlAlt(keybinding.parts[0], commandId);
        }
        this._coreKeybindings.push({
            keybinding: keybinding.parts,
            command: commandId,
            commandArgs: commandArgs,
            when: when,
            weight1: weight1,
            weight2: weight2,
            extensionId: null,
            isBuiltinExtension: false
        });
        this._cachedMergedKeybindings = null;
    }
    getDefaultKeybindings() {
        if (!this._cachedMergedKeybindings) {
            this._cachedMergedKeybindings = [].concat(this._coreKeybindings).concat(this._extensionKeybindings);
            this._cachedMergedKeybindings.sort(sorter);
        }
        return this._cachedMergedKeybindings.slice(0);
    }
}
export const KeybindingsRegistry = new KeybindingsRegistryImpl();
// Define extension point ids
export const Extensions = {
    EditorModes: 'platform.keybindingsRegistry'
};
Registry.add(Extensions.EditorModes, KeybindingsRegistry);
function sorter(a, b) {
    if (a.weight1 !== b.weight1) {
        return a.weight1 - b.weight1;
    }
    if (a.command && b.command) {
        if (a.command < b.command) {
            return -1;
        }
        if (a.command > b.command) {
            return 1;
        }
    }
    return a.weight2 - b.weight2;
}
