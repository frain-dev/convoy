/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import * as dom from '../../dom.js';
import { UILabelProvider } from '../../../common/keybindingLabels.js';
import { equals } from '../../../common/objects.js';
import './keybindingLabel.css';
import { localize } from '../../../../nls.js';
const $ = dom.$;
export class KeybindingLabel {
    constructor(container, os, options) {
        this.os = os;
        this.keyElements = new Set();
        this.options = options || Object.create(null);
        this.labelBackground = this.options.keybindingLabelBackground;
        this.labelForeground = this.options.keybindingLabelForeground;
        this.labelBorder = this.options.keybindingLabelBorder;
        this.labelBottomBorder = this.options.keybindingLabelBottomBorder;
        this.labelShadow = this.options.keybindingLabelShadow;
        this.domNode = dom.append(container, $('.monaco-keybinding'));
        this.didEverRender = false;
        container.appendChild(this.domNode);
    }
    get element() {
        return this.domNode;
    }
    set(keybinding, matches) {
        if (this.didEverRender && this.keybinding === keybinding && KeybindingLabel.areSame(this.matches, matches)) {
            return;
        }
        this.keybinding = keybinding;
        this.matches = matches;
        this.render();
    }
    render() {
        this.clear();
        if (this.keybinding) {
            const [firstPart, chordPart] = this.keybinding.getParts();
            if (firstPart) {
                this.renderPart(this.domNode, firstPart, this.matches ? this.matches.firstPart : null);
            }
            if (chordPart) {
                dom.append(this.domNode, $('span.monaco-keybinding-key-chord-separator', undefined, ' '));
                this.renderPart(this.domNode, chordPart, this.matches ? this.matches.chordPart : null);
            }
            this.domNode.title = this.keybinding.getAriaLabel() || '';
        }
        else if (this.options && this.options.renderUnboundKeybindings) {
            this.renderUnbound(this.domNode);
        }
        this.applyStyles();
        this.didEverRender = true;
    }
    clear() {
        dom.clearNode(this.domNode);
        this.keyElements.clear();
    }
    renderPart(parent, part, match) {
        const modifierLabels = UILabelProvider.modifierLabels[this.os];
        if (part.ctrlKey) {
            this.renderKey(parent, modifierLabels.ctrlKey, Boolean(match === null || match === void 0 ? void 0 : match.ctrlKey), modifierLabels.separator);
        }
        if (part.shiftKey) {
            this.renderKey(parent, modifierLabels.shiftKey, Boolean(match === null || match === void 0 ? void 0 : match.shiftKey), modifierLabels.separator);
        }
        if (part.altKey) {
            this.renderKey(parent, modifierLabels.altKey, Boolean(match === null || match === void 0 ? void 0 : match.altKey), modifierLabels.separator);
        }
        if (part.metaKey) {
            this.renderKey(parent, modifierLabels.metaKey, Boolean(match === null || match === void 0 ? void 0 : match.metaKey), modifierLabels.separator);
        }
        const keyLabel = part.keyLabel;
        if (keyLabel) {
            this.renderKey(parent, keyLabel, Boolean(match === null || match === void 0 ? void 0 : match.keyCode), '');
        }
    }
    renderKey(parent, label, highlight, separator) {
        dom.append(parent, this.createKeyElement(label, highlight ? '.highlight' : ''));
        if (separator) {
            dom.append(parent, $('span.monaco-keybinding-key-separator', undefined, separator));
        }
    }
    renderUnbound(parent) {
        dom.append(parent, this.createKeyElement(localize('unbound', "Unbound")));
    }
    createKeyElement(label, extraClass = '') {
        const keyElement = $('span.monaco-keybinding-key' + extraClass, undefined, label);
        this.keyElements.add(keyElement);
        return keyElement;
    }
    style(styles) {
        this.labelBackground = styles.keybindingLabelBackground;
        this.labelForeground = styles.keybindingLabelForeground;
        this.labelBorder = styles.keybindingLabelBorder;
        this.labelBottomBorder = styles.keybindingLabelBottomBorder;
        this.labelShadow = styles.keybindingLabelShadow;
        this.applyStyles();
    }
    applyStyles() {
        var _a;
        if (this.element) {
            for (const keyElement of this.keyElements) {
                if (this.labelBackground) {
                    keyElement.style.backgroundColor = (_a = this.labelBackground) === null || _a === void 0 ? void 0 : _a.toString();
                }
                if (this.labelBorder) {
                    keyElement.style.borderColor = this.labelBorder.toString();
                }
                if (this.labelBottomBorder) {
                    keyElement.style.borderBottomColor = this.labelBottomBorder.toString();
                }
                if (this.labelShadow) {
                    keyElement.style.boxShadow = `inset 0 -1px 0 ${this.labelShadow}`;
                }
            }
            if (this.labelForeground) {
                this.element.style.color = this.labelForeground.toString();
            }
        }
    }
    static areSame(a, b) {
        if (a === b || (!a && !b)) {
            return true;
        }
        return !!a && !!b && equals(a.firstPart, b.firstPart) && equals(a.chordPart, b.chordPart);
    }
}
