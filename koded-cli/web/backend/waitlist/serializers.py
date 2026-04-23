from rest_framework import serializers
from .models import WaitlistUser

class WaitlistUserSerializer(serializers.ModelSerializer):
    class Meta:
        model = WaitlistUser
        fields = [
            'id', 'email', 'name', 'referral_source', 
            'wants_updates', 'position', 'created_at'
        ]
        read_only_fields = ['id', 'position', 'created_at']

class WaitlistCreateSerializer(serializers.ModelSerializer):
    class Meta:
        model = WaitlistUser
        fields = ['email', 'name', 'referral_source', 'wants_updates']
    
    def validate_email(self, value):
        if WaitlistUser.objects.filter(email=value).exists():
            raise serializers.ValidationError(
                "This email is already on the waitlist."
            )
        return value
    
    def validate_name(self, value):
        if not value.strip():
            raise serializers.ValidationError("Name cannot be empty.")
        return value.strip()

class WaitlistStatsSerializer(serializers.Serializer):
    total_users = serializers.IntegerField()
    confirmed_users = serializers.IntegerField()
    last_24_hours = serializers.IntegerField()
    by_source = serializers.DictField()
    estimated_wait_days = serializers.IntegerField()