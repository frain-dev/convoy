import { AfterViewInit, Component, ElementRef, Input, OnChanges, ViewChild } from '@angular/core';
import * as Prism from 'prismjs';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-scss';
import 'prismjs/components/prism-json';
import 'prismjs/plugins/line-numbers/prism-line-numbers';

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

	constructor() {}

	ngAfterViewInit() {
		Prism.highlightElement(this.codeEle?.nativeElement);
	}

	ngOnChanges(): void {
		if (this.codeEle?.nativeElement) {
			this.codeEle.nativeElement.textContent = this.code;
			Prism.highlightElement(this.codeEle.nativeElement);
		}
	}
}
