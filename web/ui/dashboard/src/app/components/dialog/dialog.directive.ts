import { CommonModule } from '@angular/common';
import { Component, Directive, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { ButtonComponent } from '../button/button.component';

// dialog header
@Component({
	selector: '[convoy-dialog-header]',
	imports: [CommonModule, ButtonComponent],
	standalone: true,
	template: `
		<div class="px-20px pt-20px pb-16px border-y border-y-grey-10 bg-white-100 rounded-tr-16px rounded-tl-16px w-full ">
			<div class="flex justify-between items-center max-w-[770px] m-auto">
				<div class="flex items-center w-full" [ngClass]="{ 'justify-between': fullscreen === 'false' }">
					<div class="w-full" [class]="fullscreen !== 'false' ? 'order-2' : 'order-1'">
						<ng-content></ng-content>
					</div>

					<button convoy-button size="sm" texture="light" class="px-8px !py-8px" [class]="fullscreen !== 'false' ? 'order-1 mr-2' : 'order-2'" (click)="closeDialog.emit()">
						<img src="/assets/img/modal-close-icon.svg" class="w-12px h-12px" alt="close icon" />
					</button>
				</div>

				<a *ngIf="fullscreen === 'true'" convoy-button fill="text" target="_blank" href="https://getconvoy.io/docs" rel="noreferrer">
					<img src="/assets/img/doc-icon-primary.svg" alt="doc icon" />
					<span class="font-medium text-12 text-primary-100 ml-2 whitespace-nowrap">Go to docs</span>
				</a>
			</div>
		</div>
	`
})
export class DialogHeaderComponent {
	@Input('fullscreen') fullscreen: 'true' | 'false' | 'custom' = 'false';
	@Output() closeDialog = new EventEmitter();
	constructor() {}
}

@Directive({
	selector: '[convoy-dialog]',
	standalone: true,
	host: { class: 'backdrop:bg-black backdrop:bg-opacity-50 p-0 fixed top-0 left-0 right-0', '[class]': 'classes', '[id]': 'id' }
})
export class DialogDirective implements OnInit {
	@Input('position') position: 'full' | 'right' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' = 'md';
	@Input('id') id!: string;
	modalSizes = { sm: 'w-[340px]', md: 'w-[490px]', lg: 'w-[914px]' };
	modalType = {
		full: ` w-full h-full`,
		right: ` mr-0 h-full`,
		center: ` rounded-[16px] mt-180px`
	};
	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `${this.modalType[this.position]} bg-white-100 ${this.position === 'full' ? '' : this.modalSizes[this.size]}`;
	}
}
