import { Directive, Input } from '@angular/core';

@Directive({
	selector: '[convoyOverlay]',
	standalone: true,
	// Above project subnav (z-12) and default tooltips (z-50); table tooltips can opt into z-56.
	host: { class: 'fixed h-screen w-screen top-0 right-0 bottom-0 z-[55]', '[class]': "overlayHasBackdrop ? 'bg-black bg-opacity-50':''" }
})
export class OverlayDirective {
	@Input('overlayHasBackdrop') overlayHasBackdrop = false;

	constructor() {}
}
