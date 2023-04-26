/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/
var __decorate = (this && this.__decorate) || function (decorators, target, key, desc) {
    var c = arguments.length, r = c < 3 ? target : desc === null ? desc = Object.getOwnPropertyDescriptor(target, key) : desc, d;
    if (typeof Reflect === "object" && typeof Reflect.decorate === "function") r = Reflect.decorate(decorators, target, key, desc);
    else for (var i = decorators.length - 1; i >= 0; i--) if (d = decorators[i]) r = (c < 3 ? d(r) : c > 3 ? d(target, key, r) : d(target, key)) || r;
    return c > 3 && r && Object.defineProperty(target, key, r), r;
};
var __param = (this && this.__param) || function (paramIndex, decorator) {
    return function (target, key) { decorator(target, key, paramIndex); }
};
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import { DataTransfers } from '../../../../base/browser/dnd.js';
import { addDisposableListener } from '../../../../base/browser/dom.js';
import { createCancelablePromise } from '../../../../base/common/async.js';
import { createStringDataTransferItem } from '../../../../base/common/dataTransfer.js';
import { Disposable } from '../../../../base/common/lifecycle.js';
import { Mimes } from '../../../../base/common/mime.js';
import { generateUuid } from '../../../../base/common/uuid.js';
import { toVSDataTransfer, UriList } from '../../../browser/dnd.js';
import { IBulkEditService, ResourceEdit } from '../../../browser/services/bulkEditService.js';
import { Range } from '../../../common/core/range.js';
import { ILanguageFeaturesService } from '../../../common/services/languageFeatures.js';
import { EditorStateCancellationTokenSource } from '../../editorState/browser/editorState.js';
import { performSnippetEdit } from '../../snippet/browser/snippetController2.js';
import { SnippetParser } from '../../snippet/browser/snippetParser.js';
import { IClipboardService } from '../../../../platform/clipboard/common/clipboardService.js';
import { IConfigurationService } from '../../../../platform/configuration/common/configuration.js';
const vscodeClipboardMime = 'application/vnd.code.copyMetadata';
let CopyPasteController = class CopyPasteController extends Disposable {
    constructor(editor, _bulkEditService, _clipboardService, _configurationService, _languageFeaturesService) {
        super();
        this._bulkEditService = _bulkEditService;
        this._clipboardService = _clipboardService;
        this._configurationService = _configurationService;
        this._languageFeaturesService = _languageFeaturesService;
        this._editor = editor;
        const container = editor.getContainerDomNode();
        this._register(addDisposableListener(container, 'copy', e => this.handleCopy(e)));
        this._register(addDisposableListener(container, 'cut', e => this.handleCopy(e)));
        this._register(addDisposableListener(container, 'paste', e => this.handlePaste(e), true));
    }
    arePasteActionsEnabled(model) {
        return this._configurationService.getValue('editor.experimental.pasteActions.enabled', {
            resource: model.uri
        });
    }
    handleCopy(e) {
        var _a;
        if (!e.clipboardData || !this._editor.hasTextFocus()) {
            return;
        }
        const model = this._editor.getModel();
        const selections = this._editor.getSelections();
        if (!model || !(selections === null || selections === void 0 ? void 0 : selections.length)) {
            return;
        }
        if (!this.arePasteActionsEnabled(model)) {
            return;
        }
        const ranges = [...selections];
        const primarySelection = selections[0];
        const wasFromEmptySelection = primarySelection.isEmpty();
        if (wasFromEmptySelection) {
            if (!this._editor.getOption(33 /* EditorOption.emptySelectionClipboard */)) {
                return;
            }
            ranges[0] = new Range(primarySelection.startLineNumber, 0, primarySelection.startLineNumber, model.getLineLength(primarySelection.startLineNumber));
        }
        const providers = this._languageFeaturesService.documentPasteEditProvider.ordered(model).filter(x => !!x.prepareDocumentPaste);
        if (!providers.length) {
            this.setCopyMetadata(e.clipboardData, { wasFromEmptySelection });
            return;
        }
        const dataTransfer = toVSDataTransfer(e.clipboardData);
        // Save off a handle pointing to data that VS Code maintains.
        const handle = generateUuid();
        this.setCopyMetadata(e.clipboardData, {
            id: handle,
            wasFromEmptySelection,
        });
        const promise = createCancelablePromise((token) => __awaiter(this, void 0, void 0, function* () {
            const results = yield Promise.all(providers.map(provider => {
                return provider.prepareDocumentPaste(model, ranges, dataTransfer, token);
            }));
            for (const result of results) {
                result === null || result === void 0 ? void 0 : result.forEach((value, key) => {
                    dataTransfer.replace(key, value);
                });
            }
            return dataTransfer;
        }));
        (_a = this._currentClipboardItem) === null || _a === void 0 ? void 0 : _a.dataTransferPromise.cancel();
        this._currentClipboardItem = { handle: handle, dataTransferPromise: promise };
    }
    setCopyMetadata(dataTransfer, metadata) {
        dataTransfer.setData(vscodeClipboardMime, JSON.stringify(metadata));
    }
    handlePaste(e) {
        var _a, _b, _c;
        return __awaiter(this, void 0, void 0, function* () {
            if (!e.clipboardData || !this._editor.hasTextFocus()) {
                return;
            }
            const selections = this._editor.getSelections();
            if (!(selections === null || selections === void 0 ? void 0 : selections.length) || !this._editor.hasModel()) {
                return;
            }
            const model = this._editor.getModel();
            if (!this.arePasteActionsEnabled(model)) {
                return;
            }
            let metadata;
            const rawMetadata = (_a = e.clipboardData) === null || _a === void 0 ? void 0 : _a.getData(vscodeClipboardMime);
            if (rawMetadata && typeof rawMetadata === 'string') {
                metadata = JSON.parse(rawMetadata);
            }
            const providers = this._languageFeaturesService.documentPasteEditProvider.ordered(model);
            if (!providers.length) {
                return;
            }
            e.preventDefault();
            e.stopImmediatePropagation();
            const originalDocVersion = model.getVersionId();
            const tokenSource = new EditorStateCancellationTokenSource(this._editor, 1 /* CodeEditorStateFlag.Value */ | 2 /* CodeEditorStateFlag.Selection */);
            try {
                const dataTransfer = toVSDataTransfer(e.clipboardData);
                if ((metadata === null || metadata === void 0 ? void 0 : metadata.id) && ((_b = this._currentClipboardItem) === null || _b === void 0 ? void 0 : _b.handle) === metadata.id) {
                    const toMergeDataTransfer = yield this._currentClipboardItem.dataTransferPromise;
                    toMergeDataTransfer.forEach((value, key) => {
                        dataTransfer.replace(key, value);
                    });
                }
                if (!dataTransfer.has(Mimes.uriList)) {
                    const resources = yield this._clipboardService.readResources();
                    if (resources.length) {
                        dataTransfer.append(Mimes.uriList, createStringDataTransferItem(UriList.create(resources)));
                    }
                }
                dataTransfer.delete(vscodeClipboardMime);
                for (const provider of providers) {
                    if (!provider.pasteMimeTypes.some(type => {
                        if (type.toLowerCase() === DataTransfers.FILES.toLowerCase()) {
                            return [...dataTransfer.values()].some(item => item.asFile());
                        }
                        return dataTransfer.has(type);
                    })) {
                        continue;
                    }
                    const edit = yield provider.provideDocumentPasteEdits(model, selections, dataTransfer, tokenSource.token);
                    if (originalDocVersion !== model.getVersionId()) {
                        return;
                    }
                    if (edit) {
                        performSnippetEdit(this._editor, typeof edit.insertText === 'string' ? SnippetParser.escape(edit.insertText) : edit.insertText.snippet, selections);
                        if (edit.additionalEdit) {
                            yield this._bulkEditService.apply(ResourceEdit.convert(edit.additionalEdit), { editor: this._editor });
                        }
                        return;
                    }
                }
                // Default handler
                const textDataTransfer = (_c = dataTransfer.get(Mimes.text)) !== null && _c !== void 0 ? _c : dataTransfer.get('text');
                if (!textDataTransfer) {
                    return;
                }
                const text = yield textDataTransfer.asString();
                if (originalDocVersion !== model.getVersionId()) {
                    return;
                }
                this._editor.trigger('keyboard', "paste" /* Handler.Paste */, {
                    text: text,
                    pasteOnNewLine: metadata === null || metadata === void 0 ? void 0 : metadata.wasFromEmptySelection,
                    multicursorText: null
                });
            }
            finally {
                tokenSource.dispose();
            }
        });
    }
};
CopyPasteController.ID = 'editor.contrib.copyPasteActionController';
CopyPasteController = __decorate([
    __param(1, IBulkEditService),
    __param(2, IClipboardService),
    __param(3, IConfigurationService),
    __param(4, ILanguageFeaturesService)
], CopyPasteController);
export { CopyPasteController };
