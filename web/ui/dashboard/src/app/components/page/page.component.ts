import { Directive, Input } from '@angular/core';

@Directive({
	selector: 'convoy-page, [convoy-page]',
	standalone: true,
	host: { class: 'w-full m-auto', '[class]': 'types[size]' }
})
export class PageDirective {
	@Input('size') size: 'sm' | 'md' | 'lg' = 'lg';
	types = { sm: 'max-w-[848px] bg-white-100 rounded-8px mt-10', lg: 'max-w-[1374px] px-8 pb-8 pt-16', md: 'max-w-[1161px] bg-white-100 rounded-8px mt-10' };

	constructor() {}
}
