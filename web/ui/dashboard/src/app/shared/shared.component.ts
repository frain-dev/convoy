import { AfterViewInit, Component, ElementRef, Input, OnChanges, OnInit, ViewChild } from '@angular/core';
import * as Prism from 'prismjs';

@Component({
	selector: 'app-shared',
	templateUrl: './shared.component.html',
	styleUrls: ['./shared.component.scss']
})
export class SharedComponent implements AfterViewInit, OnChanges {
	@ViewChild('codeEle') codeEle!: ElementRef;
	@Input() code?: string;
	@Input() language?: string;

	constructor() {}

	ngAfterViewInit() {
		Prism.highlightElement(this.codeEle.nativeElement);
	}

	ngOnChanges(): void {
		if (this.codeEle?.nativeElement) {
			this.codeEle.nativeElement.textContent = this.code;
			Prism.highlightElement(this.codeEle.nativeElement);
		}
	}
}
