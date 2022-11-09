import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-dropdown-container, [convoy-dropdown-container]',
	standalone: true,
	host: { class: 'absolute top-[110%] w-full bg-white-100 border border-grey-10 rounded-12px shadow-default z-10 transition-all ease-in-out duration-300 h-fit', '[class]': 'classes' },
	imports: [CommonModule],
	templateUrl: './dropdown-container.component.html',
	styleUrls: ['./dropdown-container.component.scss']
})
export class DropdownContainerComponent implements OnInit {
	@Input('position') position: 'right' | 'left' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' | 'xl' | 'full' = 'md';
	@Input('show') show = false;
	@Input('className') class!: string;
	sizes = { sm: 'w-[140px]', md: 'w-[200px]', lg: 'w-[249px]', xl: 'w-[350px]', full: 'w-full' };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const positions = {
			right: 'right-[5%]',
			left: 'left-[5%]',
			center: 'left-0'
		};
		return `${this.sizes[this.size]} ${positions[this.position]} ${this.show ? 'opacity-100 h-fit overflow-y-auto pointer-events-auto' : 'opacity-0 h-0 overflow-hidden pointer-events-none'} ${this.class}`;
	}
}
