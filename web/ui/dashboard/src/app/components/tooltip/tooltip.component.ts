import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-tooltip',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './tooltip.component.html',
	styleUrls: ['./tooltip.component.scss']
})
export class TooltipComponent implements OnInit {
	@Input('size') size: 'sm' | 'md' = 'md';
	@Input('position') position: 'left' | 'right' = 'left';
	@Input('img') img!: string;
	@Input('tooltipContent') tooltipContent!: string;
	@Input('type') type?: 'primary' | 'white' = 'primary';

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `${this.position === 'right' ? '-right-[160px] after:right-[157px]' : '-right-[16px] after:right-[15px]'} ${
			this.type === 'primary' ? 'bg-primary-100 after:border-t-primary-100 bottom-[30px] text-white-100 w-192px' : 'shadow-[0px_20px_25px_-5px_rgba(51,65,85,0.1),0px_10px_10px_-5px_rgba(51,65,85,0.04)] bg-white-100 rounded-bl-[0] bottom-[calc(100%+20px)] text-black after:border-t-white-100 after:left-0 after:w-20px'
		}`;
	}
}
