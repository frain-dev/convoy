import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-resend-events',
	templateUrl: './resend-events.component.html',
	styleUrls: ['./resend-events.component.scss']
})
export class ResendEventsComponent implements OnInit {
	// Resend events form
	resendForm: FormGroup;
	isSubmitting = false;

	// Status options
	statusOptions = [
		{ uid: 'Scheduled', name: 'Scheduled' },
		{ uid: 'Processing', name: 'Processing' },
		{ uid: 'Retry', name: 'Retry' },
		{ uid: 'Failure', name: 'Failure' },
		{ uid: 'Success', name: 'Success' },
		{ uid: 'Discarded', name: 'Discarded' }
	];

	// Time interval presets
	timePresets = [
		{ uid: '30m', name: '30 minutes' },
		{ uid: '1h', name: '1 hour' },
		{ uid: '2h', name: '2 hours' },
		{ uid: '5h', name: '5 hours' },
		{ uid: '12h', name: '12 hours' },
		{ uid: '24h', name: '24 hours' },
		{ uid: 'custom', name: 'Custom' }
	];

	constructor(
		private adminService: AdminService,
		private generalService: GeneralService,
		private formBuilder: FormBuilder
	) {
		this.resendForm = this.formBuilder.group({
			status: ['Scheduled', Validators.required],
			time: ['1h', Validators.required],
			customTime: [''],
			event_id: ['']
		});

		// Add conditional validation for custom time
		this.resendForm.get('time')?.valueChanges.subscribe(value => {
			const customTimeControl = this.resendForm.get('customTime');
			if (value === 'custom') {
				customTimeControl?.setValidators([Validators.required, this.validateTimeFormat.bind(this)]);
			} else {
				customTimeControl?.clearValidators();
				customTimeControl?.setValue('');
			}
			customTimeControl?.updateValueAndValidity();
		});
	}

	ngOnInit() {
		// No initialization needed
	}

	// Custom validator for time format (Go duration format: e.g., "30m", "1h", "2h30m", "1.5h")
	validateTimeFormat(control: any) {
		if (!control.value) {
			return null;
		}
		// Go duration format: optional number (can be decimal), followed by unit (h, m, s, ms, us, µs, ns)
		// Can have multiple units like "2h30m" or "1h2m3s"
		const timePattern = /^(\d+\.?\d*[hmsµ]?s?|\d+[hmsµ]?s?)+$/;
		if (!timePattern.test(control.value.trim())) {
			return { invalidFormat: true };
		}
		return null;
	}

	// Resend events methods
	async retryEventDeliveries() {
		if (!this.resendForm.valid) {
			return;
		}

		this.isSubmitting = true;
		try {
			const formValue = this.resendForm.value;
			// Use custom time if "custom" is selected, otherwise use preset
			const timeValue = formValue.time === 'custom' ? formValue.customTime : formValue.time;
			
			await this.adminService.retryEventDeliveries({
				project_id: '',
				status: formValue.status,
				time: timeValue,
				event_id: formValue.event_id || undefined
			});
			
			this.generalService.showNotification({ 
				style: 'success', 
				message: 'Event deliveries retry initiated successfully' 
			});
			
			// Reset form
			this.resendForm.patchValue({
				status: 'Scheduled',
				time: '1h',
				customTime: '',
				event_id: ''
			});
		} catch (error) {
			console.error('Error retrying event deliveries:', error);
			this.generalService.showNotification({ 
				style: 'error', 
				message: 'Failed to retry event deliveries' 
			});
		} finally {
			this.isSubmitting = false;
		}
	}
}
