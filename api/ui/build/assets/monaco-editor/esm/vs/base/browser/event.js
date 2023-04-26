/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
import { Emitter } from '../common/event.js';
export class DomEmitter {
    constructor(element, type, useCapture) {
        const fn = (e) => this.emitter.fire(e);
        this.emitter = new Emitter({
            onFirstListenerAdd: () => element.addEventListener(type, fn, useCapture),
            onLastListenerRemove: () => element.removeEventListener(type, fn, useCapture)
        });
    }
    get event() {
        return this.emitter.event;
    }
    dispose() {
        this.emitter.dispose();
    }
}
export function stopEvent(event) {
    event.preventDefault();
    event.stopPropagation();
    return event;
}
