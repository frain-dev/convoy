import { Directive, EventEmitter, HostListener, Input, Output } from '@angular/core';

@Directive({
	selector: '[convoyOverlay]',
	standalone: true,
	host: { class: 'fixed h-screen w-screen top-0 right-0 bottom-0 z-50', '[class]': "overlayHasBackdrop ? 'bg-black bg-opacity-50':''" }
})
export class OverlayDirective {
	@Output('onClick') onClick = new EventEmitter<any>();
	@Input('overlayHasBackdrop') overlayHasBackdrop = false;

	constructor() {}

	@HostListener('click', ['$event'])
	clickEvent(event: any) {
		event.stopPropagation();
		this.onClick.emit();
	}
}
