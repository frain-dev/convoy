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
	@Input('editorValue') editorValue: any = {};
	@ViewChild('editorContainer', { static: true }) _editorContainer!: ElementRef;

	constructor(private monacoService: MonacoService) {}

	ngAfterViewInit(): void {
		this.initMonaco();
		this.monacoService.load();
	}

	private initMonaco(): void {
		if (!this.monacoService.loaded) {
			this.monacoService.loadingFinished.pipe(first()).subscribe(() => {
				this.initMonaco();
			});
			return;
		}

		this._editor = monaco.editor.create(this._editorContainer.nativeElement, {
			value: this.editorValue ? JSON.stringify(this.editorValue) : '{}',
			language: 'json',
			formatOnPaste: true,
			formatOnType: true,
			minimap: { enabled: false }
		});
	}

	// call this.monacoComponent.getValue() to get value of the editor
	public getValue(): string {
		return this._editor.getValue();
	}
}
