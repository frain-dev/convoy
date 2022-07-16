import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { AddAnalyticsService } from './add-analytics.service';

@Component({
	selector: 'app-add-analytics',
	templateUrl: './add-analytics.component.html',
	styleUrls: ['./add-analytics.component.scss']
})
export class AddAnalyticsComponent implements OnInit {
	@Output() closeModal = new EventEmitter<any>();
	@Input() authDetails: any;
	loading = false;
	addAnalyticsForm: FormGroup = this.formBuilder.group({
		is_analytics_enabled: [null, Validators.required]
	});
	constructor(private formBuilder: FormBuilder, private addAnalyticsService: AddAnalyticsService) {}

	ngOnInit(): void {}

	async addAnalytics() {
		try {
			await this.addAnalyticsService.addAnalytics(this.addAnalyticsForm.value);
			this.closeModal.emit();
		} catch {}
	}
}
