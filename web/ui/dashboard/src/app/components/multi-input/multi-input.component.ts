import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';

import { TooltipComponent } from '../tooltip/tooltip.component';
import { GeneralService } from 'src/app/services/general/general.service';

let nextInputId = 0;

@Component({
    selector: 'convoy-multi-input',
    imports: [TooltipComponent],
    templateUrl: './multi-input.component.html',
    styleUrls: ['./multi-input.component.scss']
})
export class MultiInputComponent implements OnInit {
	@Output() inputValues = new EventEmitter<string[]>();
	@Input('prefilledKeys') prefilledKeys?: string[];
	@Input('label') label?: string;
	@Input('tooltip') tooltip?: string;
	@Input('required') required: 'true' | 'false' = 'false';
	@Input('action') action!: 'view' | 'create' | 'update';
	@ViewChild('inputField', { static: true }) inputField!: ElementRef<HTMLInputElement>;

	keys: string[] = [];
	inputId = `multi-input-${nextInputId++}`;

	constructor(private generalService: GeneralService) {}

	ngOnInit(): void {
		if (this.prefilledKeys?.length) this.keys = [...this.prefilledKeys];
	}

	onInput() {
		if (this.inputField.nativeElement.value.includes(',')) this.commitPending();
	}

	onKeydown(event: KeyboardEvent) {
		if (this.action === 'view') return;
		const input = this.inputField.nativeElement;
		if (event.key === ',' || event.key === ' ' || event.key === 'Enter') {
			event.preventDefault();
			this.commitPending();
			return;
		}
		if (event.key === 'Backspace' && !input.value && this.keys.length) {
			this.keys = this.keys.slice(0, -1);
			this.inputValues.emit(this.keys);
		}
	}

	// Runs on blur so text typed without a separator is kept when the form is saved.
	commitPending() {
		const input = this.inputField.nativeElement;
		const values = input.value
			.split(',')
			.map(value => value.trim())
			.filter(Boolean);
		input.value = '';
		if (!values.length) return;

		let duplicate = false;
		for (const value of values) {
			if (this.keys.includes(value)) duplicate = true;
			else this.keys = [...this.keys, value];
		}
		if (duplicate) this.generalService.showNotification({ message: 'You have entered this key previously', style: 'warning' });
		this.inputValues.emit(this.keys);
	}

	removeKey(key: string) {
		this.keys = this.keys.filter(e => e !== key);
		this.inputValues.emit(this.keys);
	}

	focusInput() {
		this.inputField.nativeElement.focus();
	}
}
