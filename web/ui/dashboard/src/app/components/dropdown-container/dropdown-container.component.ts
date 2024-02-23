import { Directive, Input, OnInit } from '@angular/core';

@Directive({
	selector: '[convoy-dropdown-container]',
	standalone: true,
	host: { class: 'absolute w-full bg-white-100 border border-neutral-a3 rounded-12px shadow-default z-10 transition-all ease-in-out duration-300 h-fit max-h-[440px]', '[class]': 'classes' }
})
export class DropdownContainerComponent implements OnInit {
	@Input('position') position: 'right' | 'left' | 'center' | 'right-side' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' | 'xl' | 'full' = 'md';
	@Input('show') show = false;
	@Input('className') class!: string;
	sizes = { sm: 'w-140px', md: 'w-200px', lg: 'w-260px', xl: 'w-full min-w-[200px] max-w-[300px]', full: 'w-full' };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const positions = {
			right: 'top-[110%] right-[5%]',
			left: 'top-[110%] left-[5%]',
			center: 'top-[110%] left-0',
			'right-side': 'top-0 left-[105%]'
		};
		return `${this.sizes[this.size]} ${positions[this.position]} ${this.show ? 'opacity-100 h-fit pointer-events-auto overflow-y-auto overflow-x-hidden' : 'opacity-0 h-0 overflow-hidden pointer-events-none'} ${this.class}`;
	}
}
