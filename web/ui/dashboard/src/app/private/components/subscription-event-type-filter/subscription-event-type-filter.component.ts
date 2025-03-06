import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { EVENT_TYPE } from 'src/app/models/event.model';
import { FILTER, FILTER_CREATE_REQUEST } from 'src/app/models/filter.model';
import { FilterService } from '../create-subscription/filter.service';

@Component({
	selector: 'convoy-subscription-event-type-filter',
	templateUrl: './subscription-event-type-filter.component.html',
	styleUrls: ['./subscription-event-type-filter.component.scss']
})
export class SubscriptionEventTypeFilterComponent implements OnInit {
	@Input() action: 'update' | 'create' | 'view' = 'create';
	@Input() subscriptionId: string = '';
	@Input() eventTypes: EVENT_TYPE[] = [];
	@Input() selectedEventType: string = '';
	@Input() filters: FILTER[] = [];

	@Output() close = new EventEmitter<void>();
	@Output() save = new EventEmitter<FILTER[]>();

	filterForm: FormGroup = this.formBuilder.group({
		event_type: ['', Validators.required],
		headers: [{}],
		body: [{}],
		raw_headers: [{}],
		raw_body: [{}]
	});

	isLoading = false;
	showFilterEditor = false;

	constructor(private formBuilder: FormBuilder, private filterService: FilterService) {}

	ngOnInit(): void {
		if (this.selectedEventType) {
			this.filterForm.patchValue({ event_type: this.selectedEventType });
			this.showFilterEditor = true;

			// Find existing filter for this event type
			const existingFilter = this.filters.find(filter => filter.event_type === this.selectedEventType);
			if (existingFilter) {
				this.filterForm.patchValue({
					headers: existingFilter.headers || {},
					body: existingFilter.body || {},
					raw_headers: existingFilter.raw_headers || {},
					raw_body: existingFilter.raw_body || {}
				});
			}
		}
	}

	onClose(): void {
		this.close.emit();
	}

	onSave(): void {
		if (this.filterForm.invalid) {
			this.filterForm.markAllAsTouched();
			return;
		}

		const filterData: FILTER_CREATE_REQUEST = {
			subscription_id: this.subscriptionId,
			event_type: this.filterForm.value.event_type,
			headers: this.filterForm.value.headers,
			body: this.filterForm.value.body,
			raw_headers: this.filterForm.value.raw_headers,
			raw_body: this.filterForm.value.raw_body
		};

		// Update existing filters array
		const existingFilterIndex = this.filters.findIndex(filter => filter.event_type === filterData.event_type);
		if (existingFilterIndex >= 0) {
			this.filters[existingFilterIndex] = {
				...this.filters[existingFilterIndex],
				...filterData
			};
		} else {
			this.filters.push({
				id: '', // Will be assigned by backend
				subscription_id: this.subscriptionId,
				event_type: filterData.event_type,
				headers: filterData.headers || {},
				body: filterData.body || {},
				raw_headers: filterData.raw_headers || {},
				raw_body: filterData.raw_body || {},
				created_at: new Date().toISOString(),
				updated_at: new Date().toISOString()
			});
		}

		this.save.emit(this.filters);
		this.onClose();
	}

	onEventTypeChange(): void {
		this.showFilterEditor = true;

		// Find existing filter for this event type
		const existingFilter = this.filters.find(filter => filter.event_type === this.filterForm.value.event_type);
		if (existingFilter) {
			this.filterForm.patchValue({
				headers: existingFilter.headers || {},
				body: existingFilter.body || {},
				raw_headers: existingFilter.raw_headers || {},
				raw_body: existingFilter.raw_body || {}
			});
		} else {
			// Reset filter values if no existing filter
			this.filterForm.patchValue({
				headers: {},
				body: {},
				raw_headers: {},
				raw_body: {}
			});
		}
	}

	getFilterSchema(schema: any): void {
		this.filterForm.patchValue({
			headers: schema.headers || {},
			body: schema.body || {},
			raw_headers: schema.raw_headers || {},
			raw_body: schema.raw_body || {}
		});
	}
}
