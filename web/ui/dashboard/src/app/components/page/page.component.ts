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
	sizes = { small: 'max-w-[848px]', normal: 'max-w-[1374px]' };

	constructor() {}

	ngOnInit(): void {}

  get classes(): string{
    return `${this.sizes[this.size]} ${this.class} ${this.size === 'small' ? 'bg-white-100 border border-grey-10 rounded-8px mt-10' : 'px-8 pb-8 pt-16'}`
  }
}
