import { Story, Meta } from '@storybook/angular/types-6-0';
import { NotificationComponent } from '../../app/components/notification/notification.component';

export default {
	title: 'Example/Notification',
	component: NotificationComponent,
	argTypes: {
		notification: {
			message: {
                control: { type: 'text' }
            },
            style: {
                options: ['warning', 'error', 'info', 'success'],
                control: {type: 'select'}
            },
            show: {
                control: { type: 'boolean' }
            }
		}
	}
} as Meta;

const Template: Story<NotificationComponent> = (args: NotificationComponent) => ({
	props: args
});

export const Warning = Template.bind({});
Warning.args = {
	notification: {
        message: 'notification',
        style: 'warning',
        show: true
    }
};

export const Error = Template.bind({});
Error.args = {
	notification: {
        message: 'notification',
        style: 'error',
        show: true
    }
};

export const Info = Template.bind({});
Info.args = {
	notification: {
        message: 'notification',
        style: 'info',
        show: true
    }
};

export const Success = Template.bind({});
Success.args = {
	notification: {
        message: 'notification',
        style: 'success',
        show: true
    }
};

