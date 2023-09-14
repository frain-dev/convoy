import { AfterViewInit, Component, ElementRef, Input, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MonacoService } from './monaco.service';
import { first } from 'rxjs/operators';
declare var monaco: typeof import('monaco-editor');

@Component({
	selector: 'convoy-monaco',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './monaco.component.html',
	styleUrls: ['./monaco.component.scss']
})
export class MonacoComponent implements AfterViewInit {
	public _editor: any;
	private _modifiedEditor: any;
	@Input('className') class!: string;
	@Input('editorValue') editorValue: any;
	@Input('modifiedEditorValue') modifiedEditorValue: any;
	@Input('format') format: 'json' | 'javascript' | 'string' = 'json';
	@ViewChild('editorContainer', { static: true }) _editorContainer!: ElementRef;

	constructor(private monacoService: MonacoService) {}

	ngAfterViewInit(): void {
		this.initMonaco();
		this.monacoService.load();
	}

	ngOnDestroy(): void {
		if (this._editor) {
			this._editor.dispose();
		}
		if (this._modifiedEditor) {
			this._modifiedEditor.dispose();
		}
	}

	private initMonaco(): void {
		if (!this.monacoService.loaded) {
			this.monacoService.loadingFinished.pipe(first()).subscribe(() => {
				this.initMonaco();
			});
			return;
		}

		// to get color schema
		// const colors = _amdLoaderGlobal.require('vs/platform/registry/common/platform').Registry.data.get('base.contributions.colors').colorSchema.properties;
		monaco.editor.defineTheme('custom-theme', {
			base: 'vs',
			inherit: true,
			rules: [],
			colors: {
				'editor.background': '#ffffff',
				'editor.lineHighlightBorder': '#F4F4F4',
				'scrollbarSlider.background': '#ebebeb66',
				'scrollbarSlider.hoverBackground': '#e8e8e866'
			}
		});

		if (!this.modifiedEditorValue)
			this._editor = monaco.editor.create(this._editorContainer.nativeElement, {
				value: this.format == 'json' ? JSON.stringify(this.editorValue, null, '\t') : this.editorValue || '{}',
				language: this.format,
				formatOnPaste: true,
				formatOnType: true,
				minimap: { enabled: false, autohide: this.modifiedEditorValue ? false : true },
				theme: 'custom-theme'
			});

		if (this.modifiedEditorValue) {
			this._editor = monaco.editor.createModel(this.format == 'json' ? JSON.stringify(this.editorValue, null, '\t') : this.editorValue || '{}', this.format);
			this._modifiedEditor = monaco.editor.createModel(this.format == 'json' ? JSON.stringify(this.modifiedEditorValue, null, '\t') : this.modifiedEditorValue || '{}', this.format);

			const diffEditor = monaco.editor.createDiffEditor(this._editorContainer.nativeElement, {
				readOnly: true,
				renderSideBySide: true
			});

			diffEditor.setModel({
				original: this._editor,
				modified: this._modifiedEditor
			});
		}
	}

	// call this.monacoComponent.getValue() to get value of the editor
	public getValue(): string {
		return this._editor.getValue();
	}
}
