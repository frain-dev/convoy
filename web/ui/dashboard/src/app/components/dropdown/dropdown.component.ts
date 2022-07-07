import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-dropdown',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './dropdown.component.html',
	styleUrls: ['./dropdown.component.scss']
})
export class DropdownComponent implements OnInit {
	@Input('position') position: 'right' | 'left' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' | 'xl' = 'sm';
	@Input('class') class!: string;
	@Input('show') show = false;
	sizes = { sm: 'w-[140px]', md: 'w-[200px]', lg: 'w-[249px]', xl: 'w-[350px]' };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `absolute top-[110%] w-full bg-white-100 border border-grey-10 rounded-12px shadow-default z-10 transition-all ease-in-out duration-300 ${this.sizes[this.size]} ${this.position === 'right' ? 'right-[5%]' : 'left-[5%]'} ${
			this.show ? 'opacity-100 h-fit overflow-y-auto pointer-events-auto' : 'opacity-0 h-0 overflow-hidden pointer-events-none'
		} ${this.class}`;
	}


}
