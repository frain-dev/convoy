/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import { Disposable } from '../../../base/common/lifecycle.js';
import { equalsIgnoreCase, startsWithIgnoreCase } from '../../../base/common/strings.js';
import { URI } from '../../../base/common/uri.js';
import { createDecorator } from '../../instantiation/common/instantiation.js';
export const IOpenerService = createDecorator('openerService');
export const NullOpenerService = Object.freeze({
    _serviceBrand: undefined,
    registerOpener() { return Disposable.None; },
    registerValidator() { return Disposable.None; },
    registerExternalUriResolver() { return Disposable.None; },
    setDefaultExternalOpener() { },
    registerExternalOpener() { return Disposable.None; },
    open() {
        return __awaiter(this, void 0, void 0, function* () { return false; });
    },
    resolveExternalUri(uri) {
        return __awaiter(this, void 0, void 0, function* () { return { resolved: uri, dispose() { } }; });
    },
});
export function matchesScheme(target, scheme) {
    if (URI.isUri(target)) {
        return equalsIgnoreCase(target.scheme, scheme);
    }
    else {
        return startsWithIgnoreCase(target, scheme + ':');
    }
}
export function matchesSomeScheme(target, ...schemes) {
    return schemes.some(scheme => matchesScheme(target, scheme));
}
/**
 * file:///some/file.js#73
 * file:///some/file.js#L73
 * file:///some/file.js#73,84
 * file:///some/file.js#L73,84
 * file:///some/file.js#73-83
 * file:///some/file.js#L73-L83
 * file:///some/file.js#73,84-83,52
 * file:///some/file.js#L73,84-L83,52
 */
export function extractSelection(uri) {
    let selection = undefined;
    const match = /^L?(\d+)(?:,(\d+))?(-L?(\d+)(?:,(\d+))?)?/.exec(uri.fragment);
    if (match) {
        selection = {
            startLineNumber: parseInt(match[1]),
            startColumn: match[2] ? parseInt(match[2]) : 1,
            endLineNumber: match[4] ? parseInt(match[4]) : undefined,
            endColumn: match[4] ? (match[5] ? parseInt(match[5]) : 1) : undefined
        };
        uri = uri.with({ fragment: '' });
    }
    return { selection, uri };
}
