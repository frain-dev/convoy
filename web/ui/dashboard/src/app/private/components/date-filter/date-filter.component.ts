import { DatePipe } from '@angular/common';
import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'date-filter',
	templateUrl: './date-filter.component.html',
	styleUrls: ['./date-filter.component.scss']
})
export class DateFilterComponent implements OnInit {
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	showDateFilterDropdown: boolean = false;
	showOverlay: boolean = false;
	selectedDateOption!: string;
	dateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: '', disabled: true }],
		endDate: [{ value: '', disabled: true }]
	});
	@Output() selectedDateRange = new EventEmitter<any>();
	constructor(private formBuilder: FormBuilder, private datePipe: DatePipe, private generalService: GeneralService) {}

	ngOnInit(): void {}

	formatDate(date: Date) {
		return this.datePipe.transform(date, 'dd/MM/yyyy');
	}

	getSelectedDate(dateOption?: string) {
		if (dateOption) {
			this.selectedDateOption = dateOption;
			const { startDate, endDate } = this.generalService.getSelectedDate(dateOption);
			this.dateRange.patchValue({
				startDate: startDate,
				endDate: endDate
			});
			this.selectedDateRange.emit({ startDate, endDate });
		} else {
			const { startDate, endDate } = this.dateRange.value;
			this.dateRange.patchValue({
				startDate: startDate,
				endDate: endDate
			});
			this.selectedDateRange.emit({ startDate, endDate });
		}
	}

	
}
