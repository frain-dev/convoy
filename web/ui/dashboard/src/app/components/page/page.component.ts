import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-page',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './page.component.html',
	styleUrls: ['./page.component.scss']
})
export class PageComponent implements OnInit {
	@Input('size') size: 'small' | 'normal' = 'normal';
	@Input('class') class!: string;
	types = { small: 'max-w-[848px] bg-white-100 border border-grey-10 rounded-8px mt-10', normal: 'max-w-[1374px] px-8 pb-8 pt-16' };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `${this.types[this.size]} ${this.class}`;
	}
}
