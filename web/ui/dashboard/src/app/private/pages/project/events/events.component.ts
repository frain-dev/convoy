import { DatePipe } from '@angular/common';
import { Component, OnInit, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { TimeFilterComponent } from 'src/app/private/components/time-filter/time-filter.component';

@Component({
	selector: 'app-events',
	templateUrl: './events.component.html',
	styleUrls: ['./events.component.scss']
})
export class EventsComponent implements OnInit {
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	tabs: ['events', 'event deliveries'] = ['events', 'event deliveries'];
	activeTab: 'events' | 'event deliveries' = 'events';
	showOverlay: boolean = false;
	selectedDateOption: string = '';
	statsDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: new Date(new Date().setDate(new Date().getDate() - 30)), disabled: true }],
		endDate: [{ value: new Date(), disabled: true }]
	});
	
	
	constructor(private formBuilder: FormBuilder, private datePipe: DatePipe) {}

	ngOnInit(): void {}

	toggleActiveTab(tab: 'events' | 'event deliveries') {
		this.activeTab = tab;
	}

	formatDate(date: Date) {
		return this.datePipe.transform(date, 'dd/MM/yyyy');
	}
}
