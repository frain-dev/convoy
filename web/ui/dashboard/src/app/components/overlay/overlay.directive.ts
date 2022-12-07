import { Directive } from '@angular/core';

@Directive({
	selector: '[convoyOverlay]',
	standalone: true,
	host: { class: 'fixed h-screen w-screen top-0 right-0 bottom-0 z-[5]' }
})
export class OverlayDirective {
	constructor() {}
}
