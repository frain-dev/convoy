/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { illegalArgument } from '../../../base/common/errors.js';
import { AriaLabelProvider, ElectronAcceleratorLabelProvider, UILabelProvider } from '../../../base/common/keybindingLabels.js';
import { ResolvedKeybinding, ResolvedKeybindingPart } from '../../../base/common/keybindings.js';
export class BaseResolvedKeybinding extends ResolvedKeybinding {
    constructor(os, parts) {
        super();
        if (parts.length === 0) {
            throw illegalArgument(`parts`);
        }
        this._os = os;
        this._parts = parts;
    }
    getLabel() {
        return UILabelProvider.toLabel(this._os, this._parts, (keybinding) => this._getLabel(keybinding));
    }
    getAriaLabel() {
        return AriaLabelProvider.toLabel(this._os, this._parts, (keybinding) => this._getAriaLabel(keybinding));
    }
    getElectronAccelerator() {
        if (this._parts.length > 1) {
            // [Electron Accelerators] Electron cannot handle chords
            return null;
        }
        if (this._parts[0].isDuplicateModifierCase()) {
            // [Electron Accelerators] Electron cannot handle modifier only keybindings
            // e.g. "shift shift"
            return null;
        }
        return ElectronAcceleratorLabelProvider.toLabel(this._os, this._parts, (keybinding) => this._getElectronAccelerator(keybinding));
    }
    isChord() {
        return (this._parts.length > 1);
    }
    getParts() {
        return this._parts.map((keybinding) => this._getPart(keybinding));
    }
    _getPart(keybinding) {
        return new ResolvedKeybindingPart(keybinding.ctrlKey, keybinding.shiftKey, keybinding.altKey, keybinding.metaKey, this._getLabel(keybinding), this._getAriaLabel(keybinding));
    }
    getDispatchParts() {
        return this._parts.map((keybinding) => this._getDispatchPart(keybinding));
    }
    getSingleModifierDispatchParts() {
        return this._parts.map((keybinding) => this._getSingleModifierDispatchPart(keybinding));
    }
}
