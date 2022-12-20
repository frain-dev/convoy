import { Directive, Input } from '@angular/core';

@Directive({
	selector: '[convoyOverlay]',
	standalone: true,
	host: { class: 'fixed h-screen w-screen top-0 right-0 bottom-0 z-[5]', '[class]': "overlayHasBackdrop ? 'bg-black bg-opacity-50':''" }
})
export class OverlayDirective {
	@Input('overlayHasBackdrop') overlayHasBackdrop = false;

	constructor() {}
}
