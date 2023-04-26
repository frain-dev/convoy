/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { Widget } from '../widget.js';
import { CSSIcon } from '../../../common/codicons.js';
import { Color } from '../../../common/color.js';
import { Emitter } from '../../../common/event.js';
import './toggle.css';
const defaultOpts = {
    inputActiveOptionBorder: Color.fromHex('#007ACC00'),
    inputActiveOptionForeground: Color.fromHex('#FFFFFF'),
    inputActiveOptionBackground: Color.fromHex('#0E639C50')
};
export class Toggle extends Widget {
    constructor(opts) {
        super();
        this._onChange = this._register(new Emitter());
        this.onChange = this._onChange.event;
        this._onKeyDown = this._register(new Emitter());
        this.onKeyDown = this._onKeyDown.event;
        this._opts = Object.assign(Object.assign({}, defaultOpts), opts);
        this._checked = this._opts.isChecked;
        const classes = ['monaco-custom-toggle'];
        if (this._opts.icon) {
            this._icon = this._opts.icon;
            classes.push(...CSSIcon.asClassNameArray(this._icon));
        }
        if (this._opts.actionClassName) {
            classes.push(...this._opts.actionClassName.split(' '));
        }
        if (this._checked) {
            classes.push('checked');
        }
        this.domNode = document.createElement('div');
        this.domNode.title = this._opts.title;
        this.domNode.classList.add(...classes);
        if (!this._opts.notFocusable) {
            this.domNode.tabIndex = 0;
        }
        this.domNode.setAttribute('role', 'checkbox');
        this.domNode.setAttribute('aria-checked', String(this._checked));
        this.domNode.setAttribute('aria-label', this._opts.title);
        this.applyStyles();
        this.onclick(this.domNode, (ev) => {
            if (this.enabled) {
                this.checked = !this._checked;
                this._onChange.fire(false);
                ev.preventDefault();
            }
        });
        this.ignoreGesture(this.domNode);
        this.onkeydown(this.domNode, (keyboardEvent) => {
            if (keyboardEvent.keyCode === 10 /* KeyCode.Space */ || keyboardEvent.keyCode === 3 /* KeyCode.Enter */) {
                this.checked = !this._checked;
                this._onChange.fire(true);
                keyboardEvent.preventDefault();
                keyboardEvent.stopPropagation();
                return;
            }
            this._onKeyDown.fire(keyboardEvent);
        });
    }
    get enabled() {
        return this.domNode.getAttribute('aria-disabled') !== 'true';
    }
    focus() {
        this.domNode.focus();
    }
    get checked() {
        return this._checked;
    }
    set checked(newIsChecked) {
        this._checked = newIsChecked;
        this.domNode.setAttribute('aria-checked', String(this._checked));
        this.domNode.classList.toggle('checked', this._checked);
        this.applyStyles();
    }
    width() {
        return 2 /*margin left*/ + 2 /*border*/ + 2 /*padding*/ + 16 /* icon width */;
    }
    style(styles) {
        if (styles.inputActiveOptionBorder) {
            this._opts.inputActiveOptionBorder = styles.inputActiveOptionBorder;
        }
        if (styles.inputActiveOptionForeground) {
            this._opts.inputActiveOptionForeground = styles.inputActiveOptionForeground;
        }
        if (styles.inputActiveOptionBackground) {
            this._opts.inputActiveOptionBackground = styles.inputActiveOptionBackground;
        }
        this.applyStyles();
    }
    applyStyles() {
        if (this.domNode) {
            this.domNode.style.borderColor = this._checked && this._opts.inputActiveOptionBorder ? this._opts.inputActiveOptionBorder.toString() : '';
            this.domNode.style.color = this._checked && this._opts.inputActiveOptionForeground ? this._opts.inputActiveOptionForeground.toString() : 'inherit';
            this.domNode.style.backgroundColor = this._checked && this._opts.inputActiveOptionBackground ? this._opts.inputActiveOptionBackground.toString() : '';
        }
    }
    enable() {
        this.domNode.setAttribute('aria-disabled', String(false));
    }
    disable() {
        this.domNode.setAttribute('aria-disabled', String(true));
    }
}
