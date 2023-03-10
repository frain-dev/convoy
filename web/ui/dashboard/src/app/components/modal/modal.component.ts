import { CommonModule } from '@angular/common';
import { Component, Directive, EventEmitter, HostListener, Input, OnInit, Output } from '@angular/core';
import { ButtonComponent } from '../button/button.component';
import { ModalDialogComponent } from '../modal-dialog/modal-dialog.component';
import { OverlayDirective } from '../overlay/overlay.directive';


@Component({
	selector: 'convoy-modal-header, [convoy-modal-header]',
	standalone: true,
	template: `
		<div class="px-20px pt-20px pb-16px border-y border-y-grey-10 bg-white-100 rounded-tr-16px rounded-tl-16px w-full ">
			<div class="flex justify-between items-center max-w-[834px] m-auto">
				<ng-content></ng-content>
			</div>
		</div>
	`
})
export class ModalHeaderComponent {
	constructor() {}
}


@Directive({
	selector: '[convoy-modal-body]',
	standalone: true,
	host: { class: 'm-auto empty:hidden', '[class]': "position === 'full' ? 'max-w-[834px]' : 'w-full p-20px'" }
})
export class ModalBodyDirective {
	@Input('position') position: 'full' | 'left' | 'right' | 'center' = 'right';

	constructor() {}
}


@Component({
	selector: 'convoy-modal, [convoy-modal]',
	standalone: true,
	imports: [CommonModule, ButtonComponent, OverlayDirective, ModalDialogComponent, ModalBodyDirective],
	templateUrl: './modal.component.html',
	styleUrls: ['./modal.component.scss']
})
export class ModalComponent implements OnInit {
	@Input('position') position: 'full' | 'left' | 'right' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' = 'md';
	@Input('id') id!: string;
	@Output('closeModal') closeModal = new EventEmitter<any>();

	constructor() {}

	ngOnInit(): void {}

	@HostListener('document:keydown', ['$event'])
	public onFocusIn(event: any): any {
		if (event.key === 'Escape') {
			this.closeModal.emit();
		}
	}
}


