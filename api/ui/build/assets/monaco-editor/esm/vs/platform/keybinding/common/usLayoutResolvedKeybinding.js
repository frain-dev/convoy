/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { KeyCodeUtils, IMMUTABLE_CODE_TO_KEY_CODE } from '../../../base/common/keyCodes.js';
import { ChordKeybinding, SimpleKeybinding } from '../../../base/common/keybindings.js';
import { BaseResolvedKeybinding } from './baseResolvedKeybinding.js';
import { removeElementsAfterNulls } from './resolvedKeybindingItem.js';
/**
 * Do not instantiate. Use KeybindingService to get a ResolvedKeybinding seeded with information about the current kb layout.
 */
export class USLayoutResolvedKeybinding extends BaseResolvedKeybinding {
    constructor(actual, os) {
        super(os, actual.parts);
    }
    _keyCodeToUILabel(keyCode) {
        if (this._os === 2 /* OperatingSystem.Macintosh */) {
            switch (keyCode) {
                case 15 /* KeyCode.LeftArrow */:
                    return '←';
                case 16 /* KeyCode.UpArrow */:
                    return '↑';
                case 17 /* KeyCode.RightArrow */:
                    return '→';
                case 18 /* KeyCode.DownArrow */:
                    return '↓';
            }
        }
        return KeyCodeUtils.toString(keyCode);
    }
    _getLabel(keybinding) {
        if (keybinding.isDuplicateModifierCase()) {
            return '';
        }
        return this._keyCodeToUILabel(keybinding.keyCode);
    }
    _getAriaLabel(keybinding) {
        if (keybinding.isDuplicateModifierCase()) {
            return '';
        }
        return KeyCodeUtils.toString(keybinding.keyCode);
    }
    _getElectronAccelerator(keybinding) {
        return KeyCodeUtils.toElectronAccelerator(keybinding.keyCode);
    }
    _getDispatchPart(keybinding) {
        return USLayoutResolvedKeybinding.getDispatchStr(keybinding);
    }
    static getDispatchStr(keybinding) {
        if (keybinding.isModifierKey()) {
            return null;
        }
        let result = '';
        if (keybinding.ctrlKey) {
            result += 'ctrl+';
        }
        if (keybinding.shiftKey) {
            result += 'shift+';
        }
        if (keybinding.altKey) {
            result += 'alt+';
        }
        if (keybinding.metaKey) {
            result += 'meta+';
        }
        result += KeyCodeUtils.toString(keybinding.keyCode);
        return result;
    }
    _getSingleModifierDispatchPart(keybinding) {
        if (keybinding.keyCode === 5 /* KeyCode.Ctrl */ && !keybinding.shiftKey && !keybinding.altKey && !keybinding.metaKey) {
            return 'ctrl';
        }
        if (keybinding.keyCode === 4 /* KeyCode.Shift */ && !keybinding.ctrlKey && !keybinding.altKey && !keybinding.metaKey) {
            return 'shift';
        }
        if (keybinding.keyCode === 6 /* KeyCode.Alt */ && !keybinding.ctrlKey && !keybinding.shiftKey && !keybinding.metaKey) {
            return 'alt';
        }
        if (keybinding.keyCode === 57 /* KeyCode.Meta */ && !keybinding.ctrlKey && !keybinding.shiftKey && !keybinding.altKey) {
            return 'meta';
        }
        return null;
    }
    /**
     * *NOTE*: Check return value for `KeyCode.Unknown`.
     */
    static _scanCodeToKeyCode(scanCode) {
        const immutableKeyCode = IMMUTABLE_CODE_TO_KEY_CODE[scanCode];
        if (immutableKeyCode !== -1 /* KeyCode.DependsOnKbLayout */) {
            return immutableKeyCode;
        }
        switch (scanCode) {
            case 10 /* ScanCode.KeyA */: return 31 /* KeyCode.KeyA */;
            case 11 /* ScanCode.KeyB */: return 32 /* KeyCode.KeyB */;
            case 12 /* ScanCode.KeyC */: return 33 /* KeyCode.KeyC */;
            case 13 /* ScanCode.KeyD */: return 34 /* KeyCode.KeyD */;
            case 14 /* ScanCode.KeyE */: return 35 /* KeyCode.KeyE */;
            case 15 /* ScanCode.KeyF */: return 36 /* KeyCode.KeyF */;
            case 16 /* ScanCode.KeyG */: return 37 /* KeyCode.KeyG */;
            case 17 /* ScanCode.KeyH */: return 38 /* KeyCode.KeyH */;
            case 18 /* ScanCode.KeyI */: return 39 /* KeyCode.KeyI */;
            case 19 /* ScanCode.KeyJ */: return 40 /* KeyCode.KeyJ */;
            case 20 /* ScanCode.KeyK */: return 41 /* KeyCode.KeyK */;
            case 21 /* ScanCode.KeyL */: return 42 /* KeyCode.KeyL */;
            case 22 /* ScanCode.KeyM */: return 43 /* KeyCode.KeyM */;
            case 23 /* ScanCode.KeyN */: return 44 /* KeyCode.KeyN */;
            case 24 /* ScanCode.KeyO */: return 45 /* KeyCode.KeyO */;
            case 25 /* ScanCode.KeyP */: return 46 /* KeyCode.KeyP */;
            case 26 /* ScanCode.KeyQ */: return 47 /* KeyCode.KeyQ */;
            case 27 /* ScanCode.KeyR */: return 48 /* KeyCode.KeyR */;
            case 28 /* ScanCode.KeyS */: return 49 /* KeyCode.KeyS */;
            case 29 /* ScanCode.KeyT */: return 50 /* KeyCode.KeyT */;
            case 30 /* ScanCode.KeyU */: return 51 /* KeyCode.KeyU */;
            case 31 /* ScanCode.KeyV */: return 52 /* KeyCode.KeyV */;
            case 32 /* ScanCode.KeyW */: return 53 /* KeyCode.KeyW */;
            case 33 /* ScanCode.KeyX */: return 54 /* KeyCode.KeyX */;
            case 34 /* ScanCode.KeyY */: return 55 /* KeyCode.KeyY */;
            case 35 /* ScanCode.KeyZ */: return 56 /* KeyCode.KeyZ */;
            case 36 /* ScanCode.Digit1 */: return 22 /* KeyCode.Digit1 */;
            case 37 /* ScanCode.Digit2 */: return 23 /* KeyCode.Digit2 */;
            case 38 /* ScanCode.Digit3 */: return 24 /* KeyCode.Digit3 */;
            case 39 /* ScanCode.Digit4 */: return 25 /* KeyCode.Digit4 */;
            case 40 /* ScanCode.Digit5 */: return 26 /* KeyCode.Digit5 */;
            case 41 /* ScanCode.Digit6 */: return 27 /* KeyCode.Digit6 */;
            case 42 /* ScanCode.Digit7 */: return 28 /* KeyCode.Digit7 */;
            case 43 /* ScanCode.Digit8 */: return 29 /* KeyCode.Digit8 */;
            case 44 /* ScanCode.Digit9 */: return 30 /* KeyCode.Digit9 */;
            case 45 /* ScanCode.Digit0 */: return 21 /* KeyCode.Digit0 */;
            case 51 /* ScanCode.Minus */: return 83 /* KeyCode.Minus */;
            case 52 /* ScanCode.Equal */: return 81 /* KeyCode.Equal */;
            case 53 /* ScanCode.BracketLeft */: return 87 /* KeyCode.BracketLeft */;
            case 54 /* ScanCode.BracketRight */: return 89 /* KeyCode.BracketRight */;
            case 55 /* ScanCode.Backslash */: return 88 /* KeyCode.Backslash */;
            case 56 /* ScanCode.IntlHash */: return 0 /* KeyCode.Unknown */; // missing
            case 57 /* ScanCode.Semicolon */: return 80 /* KeyCode.Semicolon */;
            case 58 /* ScanCode.Quote */: return 90 /* KeyCode.Quote */;
            case 59 /* ScanCode.Backquote */: return 86 /* KeyCode.Backquote */;
            case 60 /* ScanCode.Comma */: return 82 /* KeyCode.Comma */;
            case 61 /* ScanCode.Period */: return 84 /* KeyCode.Period */;
            case 62 /* ScanCode.Slash */: return 85 /* KeyCode.Slash */;
            case 106 /* ScanCode.IntlBackslash */: return 92 /* KeyCode.IntlBackslash */;
        }
        return 0 /* KeyCode.Unknown */;
    }
    static _resolveSimpleUserBinding(binding) {
        if (!binding) {
            return null;
        }
        if (binding instanceof SimpleKeybinding) {
            return binding;
        }
        const keyCode = this._scanCodeToKeyCode(binding.scanCode);
        if (keyCode === 0 /* KeyCode.Unknown */) {
            return null;
        }
        return new SimpleKeybinding(binding.ctrlKey, binding.shiftKey, binding.altKey, binding.metaKey, keyCode);
    }
    static resolveUserBinding(input, os) {
        const parts = removeElementsAfterNulls(input.map(keybinding => this._resolveSimpleUserBinding(keybinding)));
        if (parts.length > 0) {
            return [new USLayoutResolvedKeybinding(new ChordKeybinding(parts), os)];
        }
        return [];
    }
}
