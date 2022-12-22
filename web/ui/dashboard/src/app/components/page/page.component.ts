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
	@Input('size') size: 'sm' | 'md' | 'lg' = 'lg';
	@Input('className') class!: string;
	types = { sm: 'max-w-[848px] bg-white-100 rounded-8px mt-10', lg: 'max-w-[1374px] px-8 pb-8 pt-16', md: 'max-w-[1161px] bg-white-100 rounded-8px mt-10' };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `${this.types[this.size]} ${this.class}`;
	}
}
