import { Directive, EventEmitter, HostListener, Output } from '@angular/core';

@Directive({
	selector: '[convoyScreen]',
	standalone: true,
	host: { class: 'fixed h-screen w-screen top-0 right-0 bottom-0 z-[5]' }
})
export class ScreenDirective {
    @Output('onClick') onClick = new EventEmitter<any>()
	constructor() {}

	@HostListener('click', ['$event'])
	clickEvent(event: any) {
		event.stopPropagation();
        this.onClick.emit()
	}
}
