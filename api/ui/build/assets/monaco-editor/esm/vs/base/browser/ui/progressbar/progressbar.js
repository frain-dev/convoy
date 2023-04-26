/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { show } from '../../dom.js';
import { RunOnceScheduler } from '../../../common/async.js';
import { Color } from '../../../common/color.js';
import { Disposable } from '../../../common/lifecycle.js';
import { mixin } from '../../../common/objects.js';
import './progressbar.css';
const CSS_DONE = 'done';
const CSS_ACTIVE = 'active';
const CSS_INFINITE = 'infinite';
const CSS_INFINITE_LONG_RUNNING = 'infinite-long-running';
const CSS_DISCRETE = 'discrete';
const defaultOpts = {
    progressBarBackground: Color.fromHex('#0E70C0')
};
/**
 * A progress bar with support for infinite or discrete progress.
 */
export class ProgressBar extends Disposable {
    constructor(container, options) {
        super();
        this.options = options || Object.create(null);
        mixin(this.options, defaultOpts, false);
        this.workedVal = 0;
        this.progressBarBackground = this.options.progressBarBackground;
        this.showDelayedScheduler = this._register(new RunOnceScheduler(() => show(this.element), 0));
        this.longRunningScheduler = this._register(new RunOnceScheduler(() => this.infiniteLongRunning(), ProgressBar.LONG_RUNNING_INFINITE_THRESHOLD));
        this.create(container);
    }
    create(container) {
        this.element = document.createElement('div');
        this.element.classList.add('monaco-progress-container');
        this.element.setAttribute('role', 'progressbar');
        this.element.setAttribute('aria-valuemin', '0');
        container.appendChild(this.element);
        this.bit = document.createElement('div');
        this.bit.classList.add('progress-bit');
        this.element.appendChild(this.bit);
        this.applyStyles();
    }
    off() {
        this.bit.style.width = 'inherit';
        this.bit.style.opacity = '1';
        this.element.classList.remove(CSS_ACTIVE, CSS_INFINITE, CSS_INFINITE_LONG_RUNNING, CSS_DISCRETE);
        this.workedVal = 0;
        this.totalWork = undefined;
        this.longRunningScheduler.cancel();
    }
    /**
     * Stops the progressbar from showing any progress instantly without fading out.
     */
    stop() {
        return this.doDone(false);
    }
    doDone(delayed) {
        this.element.classList.add(CSS_DONE);
        // discrete: let it grow to 100% width and hide afterwards
        if (!this.element.classList.contains(CSS_INFINITE)) {
            this.bit.style.width = 'inherit';
            if (delayed) {
                setTimeout(() => this.off(), 200);
            }
            else {
                this.off();
            }
        }
        // infinite: let it fade out and hide afterwards
        else {
            this.bit.style.opacity = '0';
            if (delayed) {
                setTimeout(() => this.off(), 200);
            }
            else {
                this.off();
            }
        }
        return this;
    }
    /**
     * Use this mode to indicate progress that has no total number of work units.
     */
    infinite() {
        this.bit.style.width = '2%';
        this.bit.style.opacity = '1';
        this.element.classList.remove(CSS_DISCRETE, CSS_DONE, CSS_INFINITE_LONG_RUNNING);
        this.element.classList.add(CSS_ACTIVE, CSS_INFINITE);
        this.longRunningScheduler.schedule();
        return this;
    }
    infiniteLongRunning() {
        this.element.classList.add(CSS_INFINITE_LONG_RUNNING);
    }
    getContainer() {
        return this.element;
    }
    style(styles) {
        this.progressBarBackground = styles.progressBarBackground;
        this.applyStyles();
    }
    applyStyles() {
        if (this.bit) {
            const background = this.progressBarBackground ? this.progressBarBackground.toString() : '';
            this.bit.style.backgroundColor = background;
        }
    }
}
/**
 * After a certain time of showing the progress bar, switch
 * to long-running mode and throttle animations to reduce
 * the pressure on the GPU process.
 *
 * https://github.com/microsoft/vscode/issues/97900
 * https://github.com/microsoft/vscode/issues/138396
 */
ProgressBar.LONG_RUNNING_INFINITE_THRESHOLD = 10000;
