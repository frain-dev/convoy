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
import { CancellationToken } from '../../../base/common/cancellation.js';
import { onUnexpectedExternalError } from '../../../base/common/errors.js';
import { URI } from '../../../base/common/uri.js';
import { IModelService } from './model.js';
import { CommandsRegistry, ICommandService } from '../../../platform/commands/common/commands.js';
import { assertType } from '../../../base/common/types.js';
import { encodeSemanticTokensDto } from './semanticTokensDto.js';
import { Range } from '../core/range.js';
import { ILanguageFeaturesService } from './languageFeatures.js';
export function isSemanticTokens(v) {
    return v && !!(v.data);
}
export function isSemanticTokensEdits(v) {
    return v && Array.isArray(v.edits);
}
export class DocumentSemanticTokensResult {
    constructor(provider, tokens, error) {
        this.provider = provider;
        this.tokens = tokens;
        this.error = error;
    }
}
export function hasDocumentSemanticTokensProvider(registry, model) {
    return registry.has(model);
}
function getDocumentSemanticTokensProviders(registry, model) {
    const groups = registry.orderedGroups(model);
    return (groups.length > 0 ? groups[0] : []);
}
export function getDocumentSemanticTokens(registry, model, lastProvider, lastResultId, token) {
    return __awaiter(this, void 0, void 0, function* () {
        const providers = getDocumentSemanticTokensProviders(registry, model);
        // Get tokens from all providers at the same time.
        const results = yield Promise.all(providers.map((provider) => __awaiter(this, void 0, void 0, function* () {
            let result;
            let error = null;
            try {
                result = yield provider.provideDocumentSemanticTokens(model, (provider === lastProvider ? lastResultId : null), token);
            }
            catch (err) {
                error = err;
                result = null;
            }
            if (!result || (!isSemanticTokens(result) && !isSemanticTokensEdits(result))) {
                result = null;
            }
            return new DocumentSemanticTokensResult(provider, result, error);
        })));
        // Try to return the first result with actual tokens or
        // the first result which threw an error (!!)
        for (const result of results) {
            if (result.error) {
                throw result.error;
            }
            if (result.tokens) {
                return result;
            }
        }
        // Return the first result, even if it doesn't have tokens
        if (results.length > 0) {
            return results[0];
        }
        return null;
    });
}
function _getDocumentSemanticTokensProviderHighestGroup(registry, model) {
    const result = registry.orderedGroups(model);
    return (result.length > 0 ? result[0] : null);
}
class DocumentRangeSemanticTokensResult {
    constructor(provider, tokens) {
        this.provider = provider;
        this.tokens = tokens;
    }
}
export function hasDocumentRangeSemanticTokensProvider(providers, model) {
    return providers.has(model);
}
function getDocumentRangeSemanticTokensProviders(providers, model) {
    const groups = providers.orderedGroups(model);
    return (groups.length > 0 ? groups[0] : []);
}
export function getDocumentRangeSemanticTokens(registry, model, range, token) {
    return __awaiter(this, void 0, void 0, function* () {
        const providers = getDocumentRangeSemanticTokensProviders(registry, model);
        // Get tokens from all providers at the same time.
        const results = yield Promise.all(providers.map((provider) => __awaiter(this, void 0, void 0, function* () {
            let result;
            try {
                result = yield provider.provideDocumentRangeSemanticTokens(model, range, token);
            }
            catch (err) {
                onUnexpectedExternalError(err);
                result = null;
            }
            if (!result || !isSemanticTokens(result)) {
                result = null;
            }
            return new DocumentRangeSemanticTokensResult(provider, result);
        })));
        // Try to return the first result with actual tokens
        for (const result of results) {
            if (result.tokens) {
                return result;
            }
        }
        // Return the first result, even if it doesn't have tokens
        if (results.length > 0) {
            return results[0];
        }
        return null;
    });
}
CommandsRegistry.registerCommand('_provideDocumentSemanticTokensLegend', (accessor, ...args) => __awaiter(void 0, void 0, void 0, function* () {
    const [uri] = args;
    assertType(uri instanceof URI);
    const model = accessor.get(IModelService).getModel(uri);
    if (!model) {
        return undefined;
    }
    const { documentSemanticTokensProvider } = accessor.get(ILanguageFeaturesService);
    const providers = _getDocumentSemanticTokensProviderHighestGroup(documentSemanticTokensProvider, model);
    if (!providers) {
        // there is no provider => fall back to a document range semantic tokens provider
        return accessor.get(ICommandService).executeCommand('_provideDocumentRangeSemanticTokensLegend', uri);
    }
    return providers[0].getLegend();
}));
CommandsRegistry.registerCommand('_provideDocumentSemanticTokens', (accessor, ...args) => __awaiter(void 0, void 0, void 0, function* () {
    const [uri] = args;
    assertType(uri instanceof URI);
    const model = accessor.get(IModelService).getModel(uri);
    if (!model) {
        return undefined;
    }
    const { documentSemanticTokensProvider } = accessor.get(ILanguageFeaturesService);
    if (!hasDocumentSemanticTokensProvider(documentSemanticTokensProvider, model)) {
        // there is no provider => fall back to a document range semantic tokens provider
        return accessor.get(ICommandService).executeCommand('_provideDocumentRangeSemanticTokens', uri, model.getFullModelRange());
    }
    const r = yield getDocumentSemanticTokens(documentSemanticTokensProvider, model, null, null, CancellationToken.None);
    if (!r) {
        return undefined;
    }
    const { provider, tokens } = r;
    if (!tokens || !isSemanticTokens(tokens)) {
        return undefined;
    }
    const buff = encodeSemanticTokensDto({
        id: 0,
        type: 'full',
        data: tokens.data
    });
    if (tokens.resultId) {
        provider.releaseDocumentSemanticTokens(tokens.resultId);
    }
    return buff;
}));
CommandsRegistry.registerCommand('_provideDocumentRangeSemanticTokensLegend', (accessor, ...args) => __awaiter(void 0, void 0, void 0, function* () {
    const [uri, range] = args;
    assertType(uri instanceof URI);
    const model = accessor.get(IModelService).getModel(uri);
    if (!model) {
        return undefined;
    }
    const { documentRangeSemanticTokensProvider } = accessor.get(ILanguageFeaturesService);
    const providers = getDocumentRangeSemanticTokensProviders(documentRangeSemanticTokensProvider, model);
    if (providers.length === 0) {
        // no providers
        return undefined;
    }
    if (providers.length === 1) {
        // straight forward case, just a single provider
        return providers[0].getLegend();
    }
    if (!range || !Range.isIRange(range)) {
        // if no range is provided, we cannot support multiple providers
        // as we cannot fall back to the one which would give results
        // => return the first legend for backwards compatibility and print a warning
        console.warn(`provideDocumentRangeSemanticTokensLegend might be out-of-sync with provideDocumentRangeSemanticTokens unless a range argument is passed in`);
        return providers[0].getLegend();
    }
    const result = yield getDocumentRangeSemanticTokens(documentRangeSemanticTokensProvider, model, Range.lift(range), CancellationToken.None);
    if (!result) {
        return undefined;
    }
    return result.provider.getLegend();
}));
CommandsRegistry.registerCommand('_provideDocumentRangeSemanticTokens', (accessor, ...args) => __awaiter(void 0, void 0, void 0, function* () {
    const [uri, range] = args;
    assertType(uri instanceof URI);
    assertType(Range.isIRange(range));
    const model = accessor.get(IModelService).getModel(uri);
    if (!model) {
        return undefined;
    }
    const { documentRangeSemanticTokensProvider } = accessor.get(ILanguageFeaturesService);
    const result = yield getDocumentRangeSemanticTokens(documentRangeSemanticTokensProvider, model, Range.lift(range), CancellationToken.None);
    if (!result || !result.tokens) {
        // there is no provider or it didn't return tokens
        return undefined;
    }
    return encodeSemanticTokensDto({
        id: 0,
        type: 'full',
        data: result.tokens.data
    });
}));
