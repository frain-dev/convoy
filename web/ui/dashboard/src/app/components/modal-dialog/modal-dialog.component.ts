import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-modal-dialog, [convoy-modal-dialog]',
	standalone: true,
	imports: [CommonModule],
	template: `
		<div class="fixed w-full shadow z-50" [id]="id" [class]="classes">
			<ng-content></ng-content>
		</div>
	`
})
export class ModalDialogComponent implements OnInit {
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
