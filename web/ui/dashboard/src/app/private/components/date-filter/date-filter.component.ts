import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';

@Component({
	selector: 'date-filter',
	templateUrl: './date-filter.component.html',
	styleUrls: ['./date-filter.component.scss']
})
export class DateFilterComponent implements OnInit {
	dateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: '', disabled: true }],
		endDate: [{ value: '', disabled: true }]
	});
	@Output() selectedDateRange = new EventEmitter<any>();
	showMatDatepicker = false;

	constructor(private formBuilder: FormBuilder) {}

	ngOnInit(): void {}

	setDate() {
		this.selectedDateRange.emit(this.dateRange.value);
	}

	clearDate() {
		this.dateRange.patchValue({ startDate: '', endDate: '' });
		this.setDate();
	}
}
