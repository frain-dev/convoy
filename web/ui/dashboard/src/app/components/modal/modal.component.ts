import { CommonModule } from '@angular/common';
import { Component, Directive, EventEmitter, HostListener, Input, OnInit, Output } from '@angular/core';
import { ButtonComponent } from '../button/button.component';
import { OverlayDirective } from '../overlay/overlay.directive';

// modal header
@Component({
	selector: '[convoy-modal-header]',
	imports: [CommonModule, ButtonComponent],
	standalone: true,
	template: `
		<div class="px-20px pt-20px pb-16px border-y border-y-grey-10 bg-white-100 rounded-tr-16px rounded-tl-16px w-full ">
			<div class="flex justify-between items-center max-w-[770px] m-auto">
				<ng-content></ng-content>

				<a *ngIf="fullscreen === 'true'" convoy-button fill="text" target="_blank" href="https://getconvoy.io/docs" rel="noreferrer">
					<img src="/assets/img/doc-icon-primary.svg" alt="doc icon" />
					<span class="font-medium text-14 text-primary-100 ml-2">Go to docs</span>
				</a>
			</div>
		</div>
	`
})
export class ModalHeaderComponent {
	@Input('fullscreen') fullscreen: 'true' | 'false' = 'false';
	constructor() {}
}

// modal dialog
@Directive({
	selector: '[convoy-modal-dialog]',
	standalone: true,
	host: { class: 'fixed w-full shadow z-50', '[class]': 'classes', '[id]': 'id' }
})
export class ModalDialogDirective implements OnInit {
	@Input('position') position: 'full' | 'left' | 'right' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' = 'md';
	@Input('id') id!: string;
	modalSizes = { sm: 'max-w-[380px]', md: 'max-w-[460px]', lg: 'max-w-[600px]' };
	modalType = {
		full: ` h-screen w-screen top-0 right-0 bottom-0 overflow-y-auto translate-x-0`,
		left: ` h-screen top-0 left-0 bottom-0 overflow-y-auto translate-x-0`,
		right: ` h-screen top-0 right-0 bottom-0 overflow-y-auto translate-x-0`,
		center: ` h-fit top-[50%] left-[50%] -translate-x-2/4 -translate-y-2/4 rounded-[16px]`
	};
	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `${this.modalType[this.position]} ${this.position === 'full' ? 'bg-[#fafafe]' : 'bg-white-100 ' + this.modalSizes[this.size]}`;
	}
}

// modal component
@Component({
	selector: '[convoy-modal]',
	standalone: true,
	imports: [CommonModule, ButtonComponent, OverlayDirective, ModalDialogDirective],
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
