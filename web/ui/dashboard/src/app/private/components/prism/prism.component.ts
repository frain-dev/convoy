import { AfterViewInit, Component, ElementRef, Input, OnChanges, ViewChild } from '@angular/core';
import * as Prism from 'prismjs';

@Component({
	selector: 'prism',
	templateUrl: './prism.component.html',
	styleUrls: ['./prism.component.scss']
})
export class PrismComponent implements AfterViewInit, OnChanges {
	@ViewChild('codeEle') codeEle!: ElementRef;
	@Input() code?: string;
	@Input() language?: string;
	@Input('title') title?: string;
	showPayload = false;
    modifiedCode?:string;

	constructor() {}

	ngAfterViewInit() {
		Prism.highlightElement(this.codeEle?.nativeElement);
	}

	ngOnChanges(): void {
		if (this.codeEle?.nativeElement) {
			this.codeEle.nativeElement.textContent = this.getCodeSnippet();
			Prism.highlightElement(this.codeEle.nativeElement);
		}
	}

    getCodeSnippet(){
        if(this.code && this.code.length > 400 && !this.showPayload) return this.code.substring(0, 400).concat('...')
        return this.code;
    }
}
