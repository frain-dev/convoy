import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-modal',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './modal.component.html',
	styleUrls: ['./modal.component.scss']
})
export class ModalComponent implements OnInit {
	@Input('position') position: 'full' | 'left' | 'right' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' = 'md';
	@Input('title') title!: string;
	@Input('id') id!: string;
	@Input('className') class!: string;
	@Output('closeModal') closeModal = new EventEmitter<any>();
	modalSizes = { sm: 'max-w-[380px]', md: 'max-w-[460px]', lg: 'max-w-[600px]' };
	modalType = {
		full: `h-screen w-screen top-0 right-0 bottom-0 overflow-y-auto translate-x-0`,
		left: `h-screen top-0 left-0 bottom-0 overflow-y-auto translate-x-0`,
		right: `h-screen top-0 right-0 bottom-0 overflow-y-auto translate-x-0`,
		center: `h-fit top-[50%] left-[50%] -translate-x-2/4 -translate-y-2/4 rounded-[16px]`
	};

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `${this.modalType[this.position]} ${this.position === 'full' ? 'bg-[#fafafe]' : this.modalSizes[this.size]} ${this.class}`;
	}
}
