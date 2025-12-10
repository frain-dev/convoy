import { Component, OnDestroy, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-resend-events',
	templateUrl: './resend-events.component.html',
	styleUrls: ['./resend-events.component.scss']
})
export class ResendEventsComponent implements OnInit, OnDestroy {
	// Resend events form
	resendForm: FormGroup;
	isSubmitting = false;
	
	// Event count
	eventCount: number | null = null;
	isLoadingCount = false;
	private countDebounceTimer: any;

	// Batch progress tracking
	currentBatchID: string | null = null;
	batchProgress: any = null;
	batchProgressList: any[] = []; // Support multiple batches
	isPolling = false;
	private pollingInterval: any;
	private notificationShownBatches = new Set<string>(); // Track which batches have shown notifications
	private batchesAtStart = new Set<string>(); // Track which batches existed when we started tracking

	// Status options
	statusOptions = [
		{ uid: 'Scheduled', name: 'Scheduled' },
		{ uid: 'Processing', name: 'Processing' },
		{ uid: 'Retry', name: 'Retry' },
		{ uid: 'Failure', name: 'Failure' },
		{ uid: 'Success', name: 'Success' },
		{ uid: 'Discarded', name: 'Discarded' },
		{ uid: 'multiple', name: 'Multiple' }
	];

	// Get status options excluding "Multiple" for the multi-select
	get multipleStatusOptions() {
		return this.statusOptions.filter(opt => opt.uid !== 'multiple');
	}

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
			multipleStatuses: [[]],
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
			this.updateEventCount();
		});

		// Add conditional validation for multiple statuses
		this.resendForm.get('status')?.valueChanges.subscribe(value => {
			const multipleStatusesControl = this.resendForm.get('multipleStatuses');
			if (value === 'multiple') {
				multipleStatusesControl?.setValidators([Validators.required, this.validateMultipleStatuses.bind(this)]);
			} else {
				multipleStatusesControl?.clearValidators();
				multipleStatusesControl?.setValue([]);
			}
			multipleStatusesControl?.updateValueAndValidity();
			this.updateEventCount();
		});

		// Update count when status or multiple statuses change
		this.resendForm.get('status')?.valueChanges.subscribe(() => {
			this.updateEventCount();
		});
		this.resendForm.get('multipleStatuses')?.valueChanges.subscribe(() => {
			this.updateEventCount();
		});

		// Update count when custom time changes
		this.resendForm.get('customTime')?.valueChanges.subscribe(() => {
			if (this.resendForm.get('time')?.value === 'custom') {
				this.updateEventCount();
			}
		});

		// Update count when event_id changes
		this.resendForm.get('event_id')?.valueChanges.subscribe(() => {
			this.updateEventCount();
		});
	}

	ngOnInit() {
		// Load initial count
		this.updateEventCount();
		
		// Load existing batches from Redis
		// Mark all existing batches as "notification shown" so we don't show notifications for old batches
		this.loadExistingBatches().then(() => {
			this.batchProgressList.forEach(batch => {
				this.notificationShownBatches.add(batch.batch_id);
				this.batchesAtStart.add(batch.batch_id);
			});
		});
	}

	ngOnDestroy() {
		this.stopPolling();
		if (this.countDebounceTimer) {
			clearTimeout(this.countDebounceTimer);
		}
	}

	// Custom validator for time format (Go duration format: e.g., "30m", "1h", "2h30m", "1.5h")
	validateTimeFormat(control: any) {
		if (!control.value) {
			return null;
		}
		// Go duration format: optional number (can be decimal), followed by unit (h, m, s, ms, us, µs, ns)
		// Can have multiple units like "2h30m" or "1h2m3s"
		const timePattern = /^(\d+(\.\d+)?(h|m|s|ms|us|µs|ns))+$/;
		if (!timePattern.test(control.value.trim())) {
			return { invalidFormat: true };
		}
		return null;
	}

	// Custom validator for multiple statuses
	validateMultipleStatuses(control: any) {
		const value = control.value;
		if (!value || !Array.isArray(value) || value.length === 0) {
			return { required: true };
		}
		return null;
	}

	// Update event count with debounce
	updateEventCount() {
		if (this.countDebounceTimer) {
			clearTimeout(this.countDebounceTimer);
		}

		this.countDebounceTimer = setTimeout(() => {
			this.fetchEventCount();
		}, 500);
	}

	async fetchEventCount() {
		if (!this.resendForm.get('status')?.value || !this.resendForm.get('time')?.value) {
			this.eventCount = null;
			return;
		}

		// Don't fetch if custom time is selected but not filled
		if (this.resendForm.get('time')?.value === 'custom' && !this.resendForm.get('customTime')?.value) {
			this.eventCount = null;
			return;
		}

		// Don't fetch if multiple statuses is selected but not filled
		if (this.resendForm.get('status')?.value === 'multiple') {
			const multipleStatuses = this.resendForm.get('multipleStatuses')?.value;
			if (!multipleStatuses || !Array.isArray(multipleStatuses) || multipleStatuses.length === 0) {
				this.eventCount = null;
				return;
			}
		}

		this.isLoadingCount = true;
		try {
			const formValue = this.resendForm.value;
			const timeValue = formValue.time === 'custom' ? formValue.customTime : formValue.time;
			
			// Determine status(es) to count - can be single or comma-separated multiple
			let statusToCount = formValue.status;
			if (formValue.status === 'multiple' && formValue.multipleStatuses?.length > 0) {
				// Extract the uid values and join them as comma-separated string
				const statusUids = formValue.multipleStatuses.map((s: any) => s.uid || s).filter((s: any) => s && s !== 'multiple');
				statusToCount = statusUids.join(',');
			}
			
			const response = await this.adminService.countRetryEventDeliveries({
				status: statusToCount,
				time: timeValue,
				event_id: formValue.event_id || undefined
			});

			this.eventCount = response.data?.num || 0;
		} catch (error) {
			console.error('Error fetching event count:', error);
			this.eventCount = null;
		} finally {
			this.isLoadingCount = false;
		}
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
			
			// Determine status(es) to send
			let statusToSend = formValue.status;
			if (formValue.status === 'multiple' && formValue.multipleStatuses?.length > 0) {
				// For multiple statuses, extract the uid values and join them as comma-separated string
				const statusUids = formValue.multipleStatuses.map((s: any) => s.uid || s).filter((s: any) => s && s !== 'multiple');
				statusToSend = statusUids.join(',');
			}
			
			const response = await this.adminService.retryEventDeliveries({
				project_id: '',
				status: statusToSend,
				time: timeValue,
				event_id: formValue.event_id || undefined
			});
			
			// Get batch ID from response and start polling
			if (response.data?.batch_id) {
				const newBatchID = response.data.batch_id;
				this.currentBatchID = newBatchID;
				
				// Mark all existing batches as "notification shown" to avoid showing notifications for old batches
				// We'll only show notifications for batches that complete after we start tracking
				this.batchProgressList.forEach(batch => {
					this.notificationShownBatches.add(batch.batch_id);
					this.batchesAtStart.add(batch.batch_id);
				});
				
				// Load existing batches to get the new one (with a small delay to ensure it's in Redis)
				setTimeout(() => {
					this.loadExistingBatches(newBatchID);
				}, 500);
				
				this.startPolling();
			}
			
			this.generalService.showNotification({ 
				style: 'success', 
				message: 'Event deliveries retry initiated in background. Processing will continue asynchronously.' 
			});
			
			// Reset form
			this.resendForm.patchValue({
				status: 'Scheduled',
				multipleStatuses: [],
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

	// Start polling for batch progress
	startPolling() {
		// Check if we have any running batches
		const runningBatches = this.batchProgressList.filter(b => b.status === 'running');
		if (runningBatches.length === 0 && !this.currentBatchID) {
			return;
		}

		if (this.isPolling) {
			return; // Already polling
		}

		this.isPolling = true;
		this.pollBatchProgress();

		// Poll every 2 seconds
		this.pollingInterval = setInterval(() => {
			this.pollBatchProgress();
		}, 2000);
	}

	// Stop polling
	stopPolling() {
		if (this.pollingInterval) {
			clearInterval(this.pollingInterval);
			this.pollingInterval = null;
		}
		this.isPolling = false;
	}

	// Poll for batch progress
	async pollBatchProgress() {
		try {
			// Refresh the list to get all batches
			await this.loadExistingBatches();
			
			// Check if we have any running batches
			const runningBatches = this.batchProgressList.filter(b => b.status === 'running');
			
			// If no running batches, stop polling
			if (runningBatches.length === 0) {
				this.stopPolling();
				
				// Show notifications only for batches that completed AFTER we started tracking
				// (i.e., batches that weren't in batchesAtStart when we began)
				this.batchProgressList.forEach(batch => {
					// Only show notification if:
					// 1. Batch is completed or failed
					// 2. We haven't shown notification for it yet
					// 3. It wasn't in the list when we started tracking (or it's the current batch we just created)
					const isNewBatch = !this.batchesAtStart.has(batch.batch_id) || batch.batch_id === this.currentBatchID;
					
					if (batch.status === 'completed' && !this.notificationShownBatches.has(batch.batch_id) && isNewBatch) {
						this.generalService.showNotification({
							style: 'success',
							message: `Batch ${batch.batch_id.substring(0, 8)}... completed. Processed ${batch.processed_count} of ${batch.total_count} events.`
						});
						this.notificationShownBatches.add(batch.batch_id);
					} else if (batch.status === 'failed' && !this.notificationShownBatches.has(batch.batch_id) && isNewBatch) {
						this.generalService.showNotification({
							style: 'error',
							message: `Batch ${batch.batch_id.substring(0, 8)}... failed: ${batch.error || 'Unknown error'}`
						});
						this.notificationShownBatches.add(batch.batch_id);
					}
				});
			}
		} catch (error: any) {
			console.error('Error polling batch progress:', error);
			// For errors, keep showing last known state and continue polling
		}
	}

	// Get progress percentage
	getProgressPercentage(batch?: any): number {
		const batchToUse = batch || this.batchProgress;
		if (!batchToUse || !batchToUse.total_count || batchToUse.total_count === 0) {
			return 0;
		}
		return Math.round((batchToUse.processed_count / batchToUse.total_count) * 100);
	}

	// Get status display text for the count message
	getStatusDisplayText(): string {
		const statusValue = this.resendForm.get('status')?.value;
		if (statusValue === 'multiple') {
			const multipleStatuses = this.resendForm.get('multipleStatuses')?.value;
			if (multipleStatuses && Array.isArray(multipleStatuses) && multipleStatuses.length > 0) {
				const statusNames = multipleStatuses.map((s: any) => s.name || s.uid || s).filter((s: any) => s && s !== 'multiple');
				return statusNames.join(', ');
			}
			return 'Multiple';
		}
		// Find the status name from options
		const statusOption = this.statusOptions.find(opt => opt.uid === statusValue);
		return statusOption?.name || statusValue;
	}

	// Dismiss batch progress (manual clear) - also deletes from Redis
	async dismissBatchProgress(batchID?: string) {
		const batchToDelete = batchID || this.currentBatchID;
		
		if (!batchToDelete) {
			return;
		}

		try {
			// Delete from Redis
			await this.adminService.deleteBatchProgress(batchToDelete);
			
			// Remove from local list
			this.batchProgressList = this.batchProgressList.filter(b => b.batch_id !== batchToDelete);
			// Remove from notification tracking
			this.notificationShownBatches.delete(batchToDelete);
			
			// If it was the current one, clear it
			if (this.currentBatchID === batchToDelete) {
				this.currentBatchID = null;
				this.batchProgress = null;
			}
			
			// If no running batches left, stop polling
			const runningBatches = this.batchProgressList.filter(b => b.status === 'running');
			if (runningBatches.length === 0) {
				this.stopPolling();
			}
		} catch (error) {
			console.error('Error deleting batch:', error);
			// Even if delete fails, remove from UI
			this.batchProgressList = this.batchProgressList.filter(b => b.batch_id !== batchToDelete);
			this.notificationShownBatches.delete(batchToDelete);
			if (this.currentBatchID === batchToDelete) {
				this.currentBatchID = null;
				this.batchProgress = null;
			}
			this.generalService.showNotification({
				style: 'error',
				message: 'Failed to delete batch from server'
			});
		}
	}

	// Load existing batches from Redis
	async loadExistingBatches(preserveCurrentBatchID?: string) {
		try {
			const response = await this.adminService.listBatchProgress();
			if (response.data && Array.isArray(response.data)) {
				// Sort by start_time descending (most recent first)
				const sortedBatches = [...response.data].sort((a, b) => {
					const timeA = new Date(a.start_time).getTime();
					const timeB = new Date(b.start_time).getTime();
					return timeB - timeA;
				});
				
				this.batchProgressList = sortedBatches;
				
				// Preserve the current batch ID if provided (when creating new batch)
				const batchIDToUse = preserveCurrentBatchID || this.currentBatchID;
				
				// Find and set current batch if provided
				if (batchIDToUse) {
					const currentBatch = this.batchProgressList.find(b => b.batch_id === batchIDToUse);
					if (currentBatch) {
						this.currentBatchID = batchIDToUse;
						this.batchProgress = currentBatch;
					}
				} else if (this.batchProgressList.length > 0) {
					// If no current batch, set the most recent as current
					const mostRecent = this.batchProgressList[0];
					this.currentBatchID = mostRecent.batch_id;
					this.batchProgress = mostRecent;
				}
				
				// Start polling if we have any running batches
				const runningBatches = this.batchProgressList.filter(b => b.status === 'running');
				if (runningBatches.length > 0 && !this.isPolling) {
					this.startPolling();
				}
			}
		} catch (error) {
			console.error('Error loading existing batches:', error);
		}
	}
}
