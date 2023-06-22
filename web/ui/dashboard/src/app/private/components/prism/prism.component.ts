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
	@Input('type') type?: 'default' | 'headers';
	showPayload = false;

	constructor() {}

	ngAfterViewInit() {
		if (this.type !== 'headers') Prism.highlightElement(this.codeEle?.nativeElement);
	}

	ngOnChanges(): void {
		if (this.codeEle?.nativeElement && this.type !== 'headers') {
			this.codeEle.nativeElement.textContent = this.code;
			Prism.highlightElement(this.codeEle.nativeElement);
		}
	}

	getHeaders() {
		if (this.type !== 'headers') return;
		let headers: any = [];
		const selectedHeaders = this.code;

		if (selectedHeaders)
			Object.entries(selectedHeaders).forEach(([key, value]) => {
				headers.push({
					header: key,
					value: Array.isArray(value) ? value[0] : value
				});
			});

		const header = {
			headersLength: headers.length,
			headers: this.showPayload ? headers : headers.slice(0, 6)
		};

		return header;
	}
}
